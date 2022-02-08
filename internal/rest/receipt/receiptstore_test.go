// Copyright 2018, 2021 Kaleido

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package receipt

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/test"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	mockreceiptapi "github.com/hyperledger/firefly-fabconnect/mocks/rest/receipt/api"
	mockws "github.com/hyperledger/firefly-fabconnect/mocks/ws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newReceiptsTestStore(replyCallback func(message interface{})) (*receiptStore, *memoryReceipts) {
	_, testConfig := test.Setup()
	r := NewReceiptStore(testConfig).(*receiptStore)
	p := newMemoryReceipts(&testConfig.Receipts)
	r.persistence = p
	return r, p
}

func TestReplyProcessorWithValidReply(t *testing.T) {
	assert := assert.New(t)

	r, p := newReceiptsTestStore(nil)

	replyMsg := &messages.TransactionReceipt{}
	replyMsg.Headers.MsgType = messages.MsgTypeTransactionSuccess
	replyMsg.Headers.ID = utils.UUIDv4()
	replyMsg.Headers.ReqID = utils.UUIDv4()
	replyMsg.Headers.ReqOffset = "topic:1:2"
	replyMsg.TransactionID = "9c842ffd430a56a5338f353a7b5b5052b4ac604564d82318af9329b4bf46dd89"
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	r.ProcessReceipt(replyMsgBytes)

	assert.Equal(1, p.receipts.Len())
	front := *p.receipts.Front().Value.(*map[string]interface{})
	assert.Equal(replyMsg.Headers.ReqID, front["_id"])

}

func TestReplyProcessorWithInvalidReplySwallowsErr(t *testing.T) {
	r, _ := newReceiptsTestStore(nil)
	r.ProcessReceipt([]byte("!json"))
}

func TestReplyProcessorWithPeristenceErrorPanics(t *testing.T) {
	r, _ := newReceiptsTestStore(nil)
	r.config.RetryTimeoutMS = 0
	p := &mockreceiptapi.ReceiptStorePersistence{}
	p.On("AddReceipt", mock.Anything, mock.Anything).Return(fmt.Errorf("bang!"))
	p.On("GetReceipt", mock.Anything).Return(nil, nil)
	r.persistence = p

	replyMsg := &messages.TransactionReceipt{}
	replyMsg.Headers.MsgType = messages.MsgTypeTransactionSuccess
	replyMsg.Headers.ID = utils.UUIDv4()
	replyMsg.Headers.ReqID = utils.UUIDv4()
	replyMsg.Headers.ReqOffset = "topic:1:2"
	replyMsg.TransactionID = "9c842ffd430a56a5338f353a7b5b5052b4ac604564d82318af9329b4bf46dd89"
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	assert.Panics(t, func() {
		r.ProcessReceipt(replyMsgBytes)
	})
}

func TestReplyProcessorWithPeristenceErrorDuplicateSwallows(t *testing.T) {
	existing := map[string]interface{}{"some": "existing"}
	r, _ := newReceiptsTestStore(nil)
	r.config.RetryTimeoutMS = 0
	p := &mockreceiptapi.ReceiptStorePersistence{}
	p.On("AddReceipt", mock.Anything, mock.Anything).Return(fmt.Errorf("bang!"))
	p.On("GetReceipt", mock.Anything).Return(&existing, nil)
	r.persistence = p

	replyMsg := &messages.TransactionReceipt{}
	replyMsg.Headers.MsgType = messages.MsgTypeTransactionSuccess
	replyMsg.Headers.ID = utils.UUIDv4()
	replyMsg.Headers.ReqID = utils.UUIDv4()
	replyMsg.Headers.ReqOffset = "topic:1:2"
	replyMsg.TransactionID = "9c842ffd430a56a5338f353a7b5b5052b4ac604564d82318af9329b4bf46dd89"
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	r.ProcessReceipt(replyMsgBytes)

	assert.True(t, p.AssertCalled(t, "AddReceipt", mock.Anything, mock.Anything))

}

func TestReplyProcessorWithErrorReply(t *testing.T) {
	assert := assert.New(t)

	r, p := newReceiptsTestStore(nil)

	replyMsg := &messages.ErrorReply{}
	replyMsg.Headers.MsgType = messages.MsgTypeError
	replyMsg.Headers.ID = utils.UUIDv4()
	replyMsg.Headers.ReqID = utils.UUIDv4()
	replyMsg.Headers.ReqOffset = "topic:1:2"
	replyMsg.OriginalMessage = "{\"badness\": true}"
	replyMsg.ErrorMessage = "pop"
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	r.ProcessReceipt(replyMsgBytes)

	assert.Equal(1, p.receipts.Len())
	front := *p.receipts.Front().Value.(*map[string]interface{})
	assert.Equal(replyMsg.Headers.ReqID, front["_id"])
	assert.Equal(replyMsg.ErrorMessage, front["errorMessage"])
	assert.Equal(replyMsg.OriginalMessage, front["requestPayload"])
}

func TestReplyProcessorMissingHeaders(t *testing.T) {
	assert := assert.New(t)

	r, p := newReceiptsTestStore(nil)

	emptyMsg := make(map[string]interface{})
	msgBytes, _ := json.Marshal(&emptyMsg)
	r.ProcessReceipt(msgBytes)

	assert.Equal(0, p.receipts.Len())
}

func TestReplyProcessorMissingRequestId(t *testing.T) {
	assert := assert.New(t)

	r, p := newReceiptsTestStore(nil)

	replyMsg := &messages.ErrorReply{}
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	r.ProcessReceipt(replyMsgBytes)

	assert.Equal(0, p.receipts.Len())
}

func TestReplyProcessorInsertError(t *testing.T) {
	assert := assert.New(t)

	r, p := newReceiptsTestStore(nil)

	replyMsg := &messages.ErrorReply{}
	replyMsg.Headers.ReqID = utils.UUIDv4()
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	r.ProcessReceipt(replyMsgBytes)

	assert.Equal(1, p.receipts.Len())
}

func TestSendReplyBroadcast(t *testing.T) {
	assert := assert.New(t)
	r, _ := newReceiptsTestStore(func(message interface{}) {
		assert.NotNil(message)
	})
	ws := &mockws.WebSocketChannels{}
	ws.On("SendReply", mock.Anything).Return()
	r.ws = ws

	replyMsg := &messages.TransactionReceipt{}
	replyMsg.Headers.MsgType = messages.MsgTypeTransactionSuccess
	replyMsg.Headers.ID = utils.UUIDv4()
	replyMsg.Headers.ReqID = utils.UUIDv4()
	replyMsg.Headers.ReqOffset = "topic:1:2"
	replyMsg.TransactionID = "9c842ffd430a56a5338f353a7b5b5052b4ac604564d82318af9329b4bf46dd89"
	replyMsgBytes, _ := json.Marshal(&replyMsg)

	r.ProcessReceipt(replyMsgBytes)
	ws.AssertCalled(t, "SendReply", mock.Anything)
}

// func TestGetRepliesCustomFiltersISO(t *testing.T) {
// 	assert := assert.New(t)
// 	_, p, ts := newReceiptsTestServer()
// 	defer ts.Close()

// 	for i := 0; i < 20; i++ {
// 		fakeReply := make(map[string]interface{})
// 		fakeReply["_id"] = fmt.Sprintf("reply%d", i)
// 		p.AddReceipt("_id", &fakeReply)
// 	}

// 	status, resObj, httpErr := testGETObject(ts, "/replies?from=abc&to=bcd&since=2019-01-01T00:00:00Z")
// 	assert.NoError(httpErr)
// 	assert.Equal(500, status)
// 	assert.Regexp("Error querying replies.*Memory receipts do not support filtering", resObj["error"])
// }

// func TestGetRepliesCustomFiltersTS(t *testing.T) {
// 	assert := assert.New(t)
// 	_, p, ts := newReceiptsTestServer()
// 	defer ts.Close()

// 	for i := 0; i < 20; i++ {
// 		fakeReply := make(map[string]interface{})
// 		fakeReply["_id"] = fmt.Sprintf("reply%d", i)
// 		p.AddReceipt("_id", &fakeReply)
// 	}

// 	status, resObj, httpErr := testGETObject(ts, "/replies?from=abc&to=bcd&since=1580435959")
// 	assert.NoError(httpErr)
// 	assert.Equal(500, status)
// 	assert.Regexp("Error querying replies.*Memory receipts do not support filtering", resObj["error"])
// }
