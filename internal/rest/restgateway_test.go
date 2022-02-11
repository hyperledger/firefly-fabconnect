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
	"github.com/hyperledger/firefly-fabconnect/internal/events"
	fabtest "github.com/hyperledger/firefly-fabconnect/internal/fabric/test"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/test"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	mockkvstore "github.com/hyperledger/firefly-fabconnect/mocks/kvstore"
	mockreceipt "github.com/hyperledger/firefly-fabconnect/mocks/rest/receipt"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/syndtr/goleveldb/leveldb"
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
	// set the mocked persistence before gateway Init() so the request handlers can use it
	testStorePersistence := &mockreceipt.ReceiptStorePersistence{}
	testStorePersistence.On("Init", mock.Anything, mock.Anything).Return(nil)
	testStorePersistence.On("ValidateConf").Return(nil)
	testStorePersistence.On("Close").Return()
	_ = g.receiptStore.Init(nil, testStorePersistence)
	err := g.Init()
	assert.NoError(err)
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

	// GET /receipts empty return
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

func TestEventsAPI(t *testing.T) {
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
	g.sm = events.NewSubscriptionManager(&g.config.Events, testRPC, nil)
	mockedKV := &mockkvstore.KVStore{}
	mockedItr := &mockkvstore.KVIterator{}
	mockedItr.On("Release").Return()
	mockedItr.On("Next").Return(false).Once() // called by recoverStreams() during Init()
	mockedItr.On("Next").Return(false).Once() // called by recoverSubscriptions() during Init()
	mockedKV.On("NewIterator").Return(mockedItr)
	_ = g.sm.Init(mockedKV)
	g.router.subManager = g.sm
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

	// POST /eventstreams success calls
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req := &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-1\",\"type\":\"webhook\",\"webhook\":{\"url\":\"https://webhook.site/abc\"}}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	result1 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result1)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(13, len(result1))
	assert.Equal(float64(1), result1["batchSize"])
	assert.Equal(float64(5000), result1["batchTimeoutMS"])
	assert.Equal("skip", result1["errorHandling"])
	assert.Equal(float64(1000), result1["timestampCacheSize"])
	assert.Equal(float64(120), result1["webhook"].(map[string]interface{})["requestTimeoutSec"])
	esID := result1["id"]

	// GET /eventstreams success calls
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodGet,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	result2 := make([]map[string]interface{}, 1)
	err = json.NewDecoder(resp.Body).Decode(&result2)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(1, len(result2))

	// GET /eventstreams/:streamId success calls
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodGet,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	result3 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result3)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(13, len(result3))

	// PATCH /eventstreams/:streamId success calls
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPatch,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-2\",\"type\":\"webhook\",\"batchSize\":5,\"errorHandling\":\"block\",\"batchTimeoutMS\":100,\"webhook\":{\"url\":\"https://webhook.site/def\"}}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	result4 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result4)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(13, len(result3))
	assert.Equal(float64(5), result4["batchSize"])
	assert.Equal(float64(100), result4["batchTimeoutMS"]) // batch timeout lowered for the resume testing in later steps
	assert.Equal("test-2", result4["name"])
	assert.Equal("block", result4["errorHandling"])
	assert.Equal("https://webhook.site/def", result4["webhook"].(map[string]interface{})["url"])

	// POST /eventstreams/:streamId/suspend failed calls due to bad ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/badId/suspend", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	var errorResp errors.RestErrMsg
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Stream with ID 'badId' not found", errorResp.Message)

	// POST /eventstreams/:streamId/suspend success calls
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s/suspend", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	result5 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result5)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(esID, result5["id"])
	assert.Equal("true", result5["suspended"])

	// POST /eventstreams/:streamId/resume failed calls due to bad ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/badId/resume", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Stream with ID 'badId' not found", errorResp.Message)

	// POST /eventstreams/:streamId/resume success calls
	time.Sleep(1 * time.Second) // sleep to allow event processor to be shutdown
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s/resume", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	result6 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result6)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(esID, result6["id"])
	assert.Equal("true", result6["resumed"])

	// POST /eventstreams failure calls due to non-json payload
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("not json"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid event stream specification: invalid character 'o' in literal null (expecting 'u')", errorResp.Message)

	// POST /eventstreams failure calls due to bad type
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-1\",\"type\":\"random\"}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Unknown action type 'random'", errorResp.Message)

	// POST /eventstreams failure calls due to missing webhook url
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-1\",\"type\":\"webhook\"}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Must specify webhook.url for action type 'webhook'", errorResp.Message)

	// POST /eventstreams failure calls due to bad webhook url
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-1\",\"type\":\"webhook\",\"webhook\":{\"url\":\":badUrl\"}}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid URL in webhook action", errorResp.Message)

	// POST /eventstreams failure calls due to bad websocket distribution mode
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-1\",\"type\":\"websocket\",\"websocket\":{\"topic\":\"apple\",\"distributionMode\":\"banana\"}}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid distribution mode 'banana'. Valid distribution modes are: 'workloadDistribution' and 'broadcast'.", errorResp.Message)

	// POST /eventstream failure calls due to DB errors
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(fmt.Errorf("bang!")).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	req.Body = ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-1\",\"type\":\"webhook\",\"webhook\":{\"url\":\"https://webhook.site/abc\"}}")))
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(500, resp.StatusCode)
	assert.Equal("Failed to store stream: bang!", errorResp.Message)

	// PATCH /eventstreams/:streamId failure calls due to invalid ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/badId", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPatch,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"updated-name\"}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Stream with ID 'badId' not found", errorResp.Message)

	// PATCH /eventstreams/:streamId failure calls due to non-json payload
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPatch,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("not json"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid event stream specification: invalid character 'o' in literal null (expecting 'u')", errorResp.Message)

	// PATCH /eventstreams/:streamId failure calls due to DB errors
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(fmt.Errorf("bang!")).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	req.Body = ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"updated-name\"}")))
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(500, resp.StatusCode)
	assert.Equal("Failed to store stream: bang!", errorResp.Message)

	// POST /subscriptions successful calls
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload := fmt.Sprintf("{\"name\":\"sub-1\",\"stream\":\"%s\",\"channel\":\"channel-1\",\"signer\":\"user1\",\"payloadType\":\"string\",\"filter\":{\"chaincodeId\":\"asset_transfer\"}}", esID)
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	result7 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result7)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(10, len(result7))
	assert.Equal("channel-1", result7["channel"])
	assert.Equal("user1", result7["signer"])
	assert.Equal("string", result7["payloadType"])
	assert.Equal("newest", result7["fromBlock"])
	assert.Equal("tx", result7["filter"].(map[string]interface{})["blockType"])
	assert.Equal(".*", result7["filter"].(map[string]interface{})["eventFilter"])
	subID := result7["id"]

	// POST /subscriptions failed calls due to non JSON payload
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("non json"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid event subscription specification: invalid character 'o' in literal null (expecting 'u')", errorResp.Message)

	// POST /subscriptions failed calls due to missing channel ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload = fmt.Sprintf("{\"name\":\"sub-1\",\"stream\":\"%s\",\"signer\":\"user1\",\"payloadType\":\"string\",\"filter\":{\"chaincodeId\":\"asset_transfer\"}}", esID)
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Missing required parameter \"channel\"", errorResp.Message)

	// POST /subscriptions failed calls due to missing stream
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload = "{\"name\":\"sub-1\",\"channel\":\"channel-1\",\"signer\":\"user1\",\"payloadType\":\"string\",\"filter\":{\"chaincodeId\":\"asset_transfer\"}}"
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Missing required parameter \"stream\"", errorResp.Message)

	// POST /subscriptions failed calls due to missing stream
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload = "{\"name\":\"sub-1\",\"channel\":\"channel-1\",\"stream\":\"test-1\",\"payloadType\":\"string\",\"filter\":{\"chaincodeId\":\"asset_transfer\"}}"
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Missing required parameter \"signer\"", errorResp.Message)

	// POST /subscriptions failed calls due to bad "fromBlock" value
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload = fmt.Sprintf("{\"name\":\"sub-1\",\"stream\":\"%s\",\"channel\":\"channel-1\",\"signer\":\"user1\",\"fromBlock\":\"0x10\"}", esID)
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid initial block: must be an integer, an empty string or 'newest'", errorResp.Message)

	// POST /subscriptions failed calls due to bad "payloadType" value
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload = fmt.Sprintf("{\"name\":\"sub-1\",\"stream\":\"%s\",\"channel\":\"channel-1\",\"signer\":\"user1\",\"payloadType\":\"badType\"}", esID)
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal(`Parameter "payloadType" must be an empty string, "string" or "json"`, errorResp.Message)

	// POST /subscriptions failed calls due to bad "blockType" value
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	payload = fmt.Sprintf("{\"name\":\"sub-1\",\"stream\":\"%s\",\"channel\":\"channel-1\",\"signer\":\"user1\",\"filter\":{\"blockType\":\"badBlockType\"}}", esID)
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte(payload))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal(`Parameter "filter.blockType" must be an empty string, "tx" or "config"`, errorResp.Message)

	// GET /subscriptions success calls
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodGet,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	result8 := make([]map[string]interface{}, 1)
	err = json.NewDecoder(resp.Body).Decode(&result8)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(1, len(result8))

	// GET /subscriptions/:subId failed calls due to bad ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/badId", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodGet,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Subscription with ID 'badId' not found", errorResp.Message)

	// GET /subscriptions/:subId success calls
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/%s", g.config.HTTP.Port, subID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodGet,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	result9 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result9)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(10, len(result9))

	// POST /subscriptions/:subId/reset success calls
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/%s/reset", g.config.HTTP.Port, subID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"initialBlock\":\"100\"}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	result10 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result10)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(subID, result10["id"])
	assert.Equal("true", result10["reset"])

	// POST /subscriptions/:subId/reset failed calls due to bad ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/badId/reset", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"initialBlock\":\"100\"}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Subscription with ID 'badId' not found", errorResp.Message)

	// POST /subscriptions/:subId/reset failed calls due to bad payload
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/%s/reset", g.config.HTTP.Port, subID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("not json"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Failed to parse request body. invalid character 'o' in literal null (expecting 'u')", errorResp.Message)

	// POST /subscriptions/:subId/reset failed calls due to bad payload
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/%s/reset", g.config.HTTP.Port, subID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"initialBlock\":\"0x10\"}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(400, resp.StatusCode)
	assert.Equal("Invalid initial block: must be an integer, an empty string or 'newest'", errorResp.Message)

	// DELETE /subscriptions/:subId failed calls due to bad ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/badId", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodDelete,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Subscription with ID 'badId' not found", errorResp.Message)

	// DELETE /subscriptions/:subId success calls
	mockedKV.On("Delete", mock.Anything).Return(nil).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/subscriptions/%s", g.config.HTTP.Port, subID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodDelete,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	result11 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result11)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(subID, result11["id"])
	assert.Equal("true", result11["deleted"])

	// DELETE /eventstreams/:streamId failure calls due to bad ID
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/badId", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodDelete,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(404, resp.StatusCode)
	assert.Equal("Stream with ID 'badId' not found", errorResp.Message)

	// DELETE /eventstreams/:streamId failure calls due to DB errors
	mockedKV.On("Delete", mock.Anything).Return(fmt.Errorf("bang!")).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodDelete,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	assert.Equal(500, resp.StatusCode)
	assert.Equal("bang!", errorResp.Message)

	// DELETE /eventstreams/:streamId success calls
	mockedKV.On("Put", mock.Anything, mock.Anything).Return(nil).Once()
	mockedKV.On("Get", mock.Anything).Return([]byte{}, leveldb.ErrNotFound).Once()
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams", g.config.HTTP.Port))
	req = &http.Request{
		URL:    url,
		Method: http.MethodPost,
		Header: header,
		Body:   ioutil.NopCloser(bytes.NewReader([]byte("{\"name\":\"test-2\",\"type\":\"webhook\",\"webhook\":{\"url\":\"https://webhook.site/abc\"}}"))),
	}
	resp, err = http.DefaultClient.Do(req)
	result12 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result12)
	esID = result12["id"]
	mockedKV.On("Delete", mock.Anything).Return(nil).Twice() // once for stream, once for checkpoint
	url, _ = url.Parse(fmt.Sprintf("http://localhost:%d/eventstreams/%s", g.config.HTTP.Port, esID))
	req = &http.Request{
		URL:    url,
		Method: http.MethodDelete,
		Header: header,
	}
	resp, err = http.DefaultClient.Do(req)
	result13 := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&result13)
	assert.Equal(200, resp.StatusCode)
	assert.Equal(esID, result13["id"])
	assert.Equal("true", result13["deleted"])
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
	assert.EqualError(err, "User credentials store creation failed. User credentials store path is empty")
}
