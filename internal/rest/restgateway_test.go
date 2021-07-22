// Copyright 2021 Kaleido

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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/auth"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/auth/authtest"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest/test"
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
	g := NewRESTGateway(testConfig)
	g.config.HTTP.Port = lastPort
	g.config.HTTP.LocalAddr = "127.0.0.1"
	g.config.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
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

func TestStartStatusStopNoKafkaHandlerMissingToken(t *testing.T) {
	assert := assert.New(t)

	auth.RegisterSecurityModule(&authtest.TestSecurityModule{})

	g := NewRESTGateway(testConfig)
	g.config.HTTP.Port = lastPort
	g.config.HTTP.LocalAddr = "127.0.0.1"
	g.config.RPC.ConfigPath = path.Join(tmpdir, "ccp.yml")
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

func TestStartWithKafkaHandlerNoBroker(t *testing.T) {
	assert := assert.New(t)

	g := NewRESTGateway(testConfig)
	g.config.HTTP.Port = lastPort
	g.config.HTTP.LocalAddr = "127.0.0.1"
	g.config.Kafka.Brokers = []string{""}
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
	assert.EqualError(err, "No Kafka brokers configured")
}

func TestStartWithBadTLS(t *testing.T) {
	assert := assert.New(t)

	g := NewRESTGateway(testConfig)
	g.config.HTTP.Port = lastPort
	g.config.HTTP.LocalAddr = "127.0.0.1"
	g.config.HTTP.TLS.Enabled = true
	g.config.HTTP.TLS.ClientKeyFile = "incomplete config"
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

	g := NewRESTGateway(testConfig)
	url, _ := url.Parse(fakeMongo.URL)
	url.Scheme = "mongodb"
	g.config.Receipts.LevelDB.Path = ""
	g.config.Receipts.MongoDB.URL = url.String()
	g.config.Receipts.MongoDB.Database = "test"
	g.config.Receipts.MongoDB.Collection = "test"
	g.config.Receipts.MongoDB.ConnectTimeoutMS = 100
	g.config.HTTP.TLS.ClientKeyFile = ""
	err := g.Init()
	assert.EqualError(err, "Unable to connect to MongoDB: no reachable servers")
}

func TestStartWithBadRPCConfigPath(t *testing.T) {
	assert := assert.New(t)

	g := NewRESTGateway(testConfig)
	g.config.HTTP.Port = lastPort
	g.config.HTTP.LocalAddr = "127.0.0.1"
	g.config.RPC.ConfigPath = "/bad/path"
	err := g.Init()
	assert.EqualError(err, "open /bad/path: no such file or directory")
}
