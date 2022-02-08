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
	mockreceipt "github.com/hyperledger/firefly-fabconnect/mocks/rest/receipt"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestEventstreamAPIErrors(t *testing.T) {
	assert := assert.New(t)

	auth.RegisterSecurityModule(&authtest.TestSecurityModule{})
	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
	testConfig.Events.LevelDB.Path = tmpdir
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

	url, _ := url.Parse(fmt.Sprintf("http://localhost:%d/status", g.config.HTTP.Port))
	var resp *http.Response
	header := http.Header{
		"AUTHORIZATION": []string{"BeaRER testat"},
	}
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		req := &http.Request{URL: url, Method: http.MethodGet, Header: header}
		resp, err = http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			break
		}
	}
	assert.NoError(err)
	assert.Equal(200, resp.StatusCode)

	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/badId", g.config.HTTP.Port))
	req := &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	var errorResp errors.RestErrMsg
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Stream with ID 'badId' not found", errorResp.Message)

	g.srv.Close()
	wg.Wait()
	assert.EqualError(err, "http: Server closed")
	auth.RegisterSecurityModule(nil)
}

func TestReceiptsAPI(t *testing.T) {
	assert := assert.New(t)

	auth.RegisterSecurityModule(&authtest.TestSecurityModule{})
	testConfig.HTTP.Port = lastPort
	testConfig.HTTP.LocalAddr = "127.0.0.1"
	testConfig.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
	testConfig.Events.LevelDB.Path = ""
	testConfig.Receipts.LevelDB.Path = ""
	g := NewRESTGateway(testConfig)
	err := g.Init()
	assert.NoError(err)
	testRPC := fabtest.MockRPCClient("")
	g.processor.Init(testRPC)
	testStorePersistence := &mockreceipt.ReceiptStorePersistence{}
	testStorePersistence.On("Close").Return()
	err = g.receiptStore.Init(nil, testStorePersistence)
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
	header := http.Header{
		"AUTHORIZATION": []string{"BeaRER testat"},
	}
	for i := 0; i < 5; i++ {
		time.Sleep(200 * time.Millisecond)
		req := &http.Request{URL: url, Method: http.MethodGet, Header: header}
		resp, err = http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			break
		}
	}
	assert.NoError(err)
	assert.Equal(200, resp.StatusCode)

	// GET /receipts emptry return
	fakeReply1 := make([]map[string]interface{}, 0)
	testStorePersistence.On("GetReceipts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&fakeReply1, nil).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts", g.config.HTTP.Port))
	req := &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	result1 := make([]map[string]interface{}, 0)
	err = json.NewDecoder(resp.Body).Decode(&result1)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(0, len(result1))

	// GET /receipts returns with default limit
	var fakeReplies []map[string]interface{}
	testStorePersistence.On("GetReceipts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&fakeReplies, nil).Once()
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(200, resp.StatusCode)
	defaultReceiptLimit := 10 // from the package internal/rest/receipt
	testStorePersistence.AssertCalled(t, "GetReceipts", 0, defaultReceiptLimit, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// GET /receipts returns with custom skip and limit
	testStorePersistence.On("GetReceipts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&fakeReplies, nil).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts?skip=5&limit=20", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	assert.Equal(200, resp.StatusCode)
	testStorePersistence.AssertCalled(t, "GetReceipts", 5, 20, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	// GET /receipts error on bad limit parameters
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts?limit=bad&skip=10", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	var errorResp errors.RestErrMsg
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid 'limit' query parameter", errorResp.Message)

	// GET /receipts error on excessive limit parameters
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts?limit=1000&skip=10", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Maximum limit is 100", errorResp.Message)

	// GET /receipts error on bad skip parameters
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts?limit=10&skip=bad", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid 'skip' query parameter", errorResp.Message)

	// GET /receipts error on bad filtering parameters
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts?from=abc&to=bcd&since=badness", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("since cannot be parsed as RFC3339 or millisecond timestamp", errorResp.Message)

	// GET /receipts error on DB errors
	testStorePersistence.On("GetReceipts", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("bang!")).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(500, resp.StatusCode)
	assert.Equal("Error querying replies: bang!", errorResp.Message)

	// GET /receipts error on invalid query parameters
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts?id=!!!", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid 'id' query parameter", errorResp.Message)

	// GET /receipt/:receiptId successful return
	fakeReply2 := make(map[string]interface{})
	fakeReply2["_id"] = "ABCDEFG"
	fakeReply2["field1"] = "value1"
	testStorePersistence.On("GetReceipt", mock.Anything).Return(&fakeReply2, nil).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts/b59d7c35-189b-44bd-a490-9cb472466f19", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	result := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result)
	assert.Equal(200, resp.StatusCode)
	assert.Equal("value1", result["field1"])

	// GET /receipt/:receiptId 500 error on DB query error
	testStorePersistence.On("GetReceipt", mock.Anything).Return(nil, fmt.Errorf("not found!")).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/receipts/badId", g.config.HTTP.Port))
	req = &http.Request{URL: url, Method: http.MethodGet, Header: header}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(500, resp.StatusCode)
	assert.Equal("Error querying reply: not found!", errorResp.Message)

	// GET /receipt/:receiptId 404 error on DB query not found
	testStorePersistence.On("GetReceipt", mock.Anything).Return(nil, nil).Once()
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)

	// GET /receipt/:receiptId 500 error on invalid DB return
	fakeReply3 := make(map[string]interface{})
	fakeReply3["_id"] = "ABCDEFG"
	unserializable := make(map[interface{}]interface{})
	unserializable[true] = "not for json"
	fakeReply3["badness"] = unserializable
	testStorePersistence.On("GetReceipt", mock.Anything).Return(&fakeReply3, nil).Once()
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(500, resp.StatusCode)
	assert.Equal("Error serializing response", errorResp.Message)

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
