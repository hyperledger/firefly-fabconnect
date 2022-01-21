// Copyright 2021 Kaleido
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/auth"
	"github.com/hyperledger/firefly-fabconnect/internal/auth/authtest"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	fabtest "github.com/hyperledger/firefly-fabconnect/internal/fabric/test"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/test"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

var lastPort = 9000
var tmpdir string
var testConfig *conf.RESTGatewayConf

func TestMain(m *testing.M) {
	setup()
	code := m.Run()
	teardown()
	os.Exit(code)
}

func setup() {
	tmpdir, testConfig = test.Setup()
}

func teardown() {
	test.Teardown(tmpdir)
}

func TestNewRESTGateway(t *testing.T) {
	assert := assert.New(t)
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	g := NewRESTGateway(testConfig)
	assert.Equal("127.0.0.1", g.config.HTTP.LocalAddr)
}

func TestStartStatusStopNoKafkaHandlerAccessToken(t *testing.T) {
	assert := assert.New(t)

	auth.RegisterSecurityModule(&authtest.TestSecurityModule{})
	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
	g := NewRESTGateway(testConfig)
	err := g.Init()
	assert.NoError(err)

	lastPort++
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = g.Start()
		wg.Done()
	}()

	url, _ := url.Parse(fmt.Sprintf("http://localhost:%d/status", g.config.HTTP.Port))
	var resp *http.Response
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		req := &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
			"AUTHORIZATION": []string{"BeaRER testat"},
		}}
		resp, err = http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			break
		}
	}
	assert.NoError(err)
	assert.Equal(200, resp.StatusCode)
	var statusResp statusMsg
	err = json.NewDecoder(resp.Body).Decode(&statusResp)
	assert.Equal(true, statusResp.OK)

	g.srv.Close()
	wg.Wait()
	assert.EqualError(err, "http: Server closed")

	auth.RegisterSecurityModule(nil)

}

func TestQueryEndpoints(t *testing.T) {
	assert := assert.New(t)

	auth.RegisterSecurityModule(&authtest.TestSecurityModule{})

	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
	g := NewRESTGateway(testConfig)
	err := g.Init()
	testRPC := fabtest.MockRPCClient("")
	g.processor.Init(testRPC)
	assert.NoError(err)

	lastPort++
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = g.Start()
		wg.Done()
	}()

	url, _ := url.Parse(fmt.Sprintf("http://localhost:%d/chainInfo?fly-channel=default-channel&fly-signer=user1", g.config.HTTP.Port))
	var resp *http.Response
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		req := &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
			"authorization": []string{"bearer testat"},
		}}
		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			break
		}
	}
	assert.NoError(err)
	assert.Equal(200, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	result := utils.DecodePayload(bodyBytes).(map[string]interface{})
	rr := result["result"].(map[string]interface{})
	assert.Equal(float64(10), rr["height"])

	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/chainInfo", g.config.HTTP.Port))
	req := &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
		"authorization": []string{"bearer testat"},
	}}
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(400, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	assert.Equal("{\"error\":\"Must specify the channel\"}", string(bodyBytes))

	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/chainInfo?fly-channel=default-channel", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
		"authorization": []string{"bearer testat"},
	}}
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(400, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	assert.Equal("{\"error\":\"Must specify the signer\"}", string(bodyBytes))

	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/transactions/3144a3ad43dcc11374832bbb71561320de81fd80d69cc8e26a9ea7d3240a5e84?fly-channel=default-channel&fly-signer=user1", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
		"authorization": []string{"bearer testat"},
	}}
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(200, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	result = utils.DecodePayload(bodyBytes).(map[string]interface{})
	rr = result["result"].(map[string]interface{})
	tx := rr["transaction"].(map[string]interface{})
	assert.Equal("3144a3ad43dcc11374832bbb71561320de81fd80d69cc8e26a9ea7d3240a5e84", tx["tx_id"])
	assert.Equal(float64(1000000), tx["timestamp"])

	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/blocks/20?fly-channel=default-channel&fly-signer=user1", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
		"authorization": []string{"bearer testat"},
	}}
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(200, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	result = utils.DecodePayload(bodyBytes).(map[string]interface{})
	rr = result["result"].(map[string]interface{})
	block := rr["block"].(map[string]interface{})
	assert.Equal(float64(20), block["block_number"])

	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/query?fly-channel=default-channel&fly-signer=user1&fly-chaincode=asset_transfer", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: http.Header{"authorization": []string{"bearer testat"}},
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(400, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	assert.Equal("{\"error\":\"Must specify target chaincode function\"}", string(bodyBytes))

	req.Body = ioutil.NopCloser(bytes.NewReader([]byte("{\"func\":\"\"}")))
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(400, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	assert.Equal("{\"error\":\"Target chaincode function must not be empty\"}", string(bodyBytes))

	req.Body = ioutil.NopCloser(bytes.NewReader([]byte("{\"func\":\"CreateAsset\"}")))
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(400, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	assert.Equal("{\"error\":\"must specify args\"}", string(bodyBytes))

	req.Body = ioutil.NopCloser(bytes.NewReader([]byte("{\"func\":\"CreateAsset\",\"args\":[]}")))
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(200, resp.StatusCode)
	bodyBytes, _ = io.ReadAll(resp.Body)
	result = utils.DecodePayload(bodyBytes).(map[string]interface{})
	rr = result["result"].(map[string]interface{})
	assert.Equal(float64(123000), rr["AppraisedValue"])
	assert.Equal("asset01", rr["ID"])

	g.srv.Close()
	wg.Wait()
	assert.EqualError(err, "http: Server closed")

	auth.RegisterSecurityModule(nil)

}

func TestStartStatusStopNoKafkaHandlerMissingToken(t *testing.T) {
	assert := assert.New(t)

	auth.RegisterSecurityModule(&authtest.TestSecurityModule{})

	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
	g := NewRESTGateway(testConfig)
	err := g.Init()
	assert.NoError(err)

	lastPort++
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = g.Start()
		wg.Done()
	}()

	url, _ := url.Parse(fmt.Sprintf("http://localhost:%d/status", g.config.HTTP.Port))
	var resp *http.Response
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		req := &http.Request{URL: url, Method: http.MethodGet, Header: http.Header{
			"authorization": []string{"bearer"},
		}}
		resp, err = http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == 401 {
			break
		}
	}
	assert.NoError(err)
	assert.Equal(401, resp.StatusCode)
	var errResp errors.RestErrMsg
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	assert.Equal("Unauthorized", errResp.Message)

	g.srv.Close()
	wg.Wait()
	assert.EqualError(err, "http: Server closed")

	auth.RegisterSecurityModule(nil)

}

func TestStartWithBadTLS(t *testing.T) {
	assert := assert.New(t)

	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.HTTP.TLS.Enabled = true
	testConfig.HTTP.TLS.ClientKeyFile = "incomplete config"
	g := NewRESTGateway(testConfig)
	err := g.Init()
	assert.NoError(err)

	lastPort++
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = g.Start()
		wg.Done()
	}()

	wg.Wait()
	assert.EqualError(err, "Client private key and certificate must both be provided for mutual auth")
}

func TestStartInvalidMongo(t *testing.T) {
	assert := assert.New(t)

	fakeRouter := &httprouter.Router{}
	fakeMongo := httptest.NewServer(fakeRouter)
	defer fakeMongo.Close()

	url, _ := url.Parse(fakeMongo.URL)
	url.Scheme = "mongodb"
	testConfig.Receipts.LevelDB.Path = ""
	testConfig.Receipts.MongoDB.URL = url.String()
	testConfig.Receipts.MongoDB.Database = "test"
	testConfig.Receipts.MongoDB.Collection = "test"
	testConfig.Receipts.MongoDB.ConnectTimeoutMS = 100
	testConfig.HTTP.TLS.ClientKeyFile = ""
	g := NewRESTGateway(testConfig)
	err := g.Init()
	assert.EqualError(err, "Unable to connect to MongoDB: no reachable servers")
}

func TestStartWithMissingUserStorePath(t *testing.T) {
	assert := assert.New(t)

	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.RPC.ConfigPath = ""
	g := NewRESTGateway(testConfig)
	err := g.Init()
	assert.EqualError(err, "User credentials store creation failed. Path: User credentials store path is empty")
}
