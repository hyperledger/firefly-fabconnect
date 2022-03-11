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

package sync

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	restutil "github.com/hyperledger/firefly-fabconnect/internal/rest/utils"
	"github.com/hyperledger/firefly-fabconnect/internal/tx"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/julienschmidt/httprouter"

	log "github.com/sirupsen/logrus"
)

// SyncDispatcher abstracts the processing of the transactions and queries
// synchronously. We perform those within this package.
type SyncDispatcher interface {
	DispatchMsgSync(ctx context.Context, res http.ResponseWriter, req *http.Request, msg interface{})
	QueryChaincode(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	GetTxById(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	GetChainInfo(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	GetBlock(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	GetBlockByTxId(res http.ResponseWriter, req *http.Request, params httprouter.Params)
}

type syncDispatcher struct {
	processor tx.TxProcessor
}

func NewSyncDispatcher(processor tx.TxProcessor) SyncDispatcher {
	return &syncDispatcher{
		processor: processor,
	}
}

type syncTxInflight struct {
	ctx            context.Context
	replyProcessor *syncResponder
	timeReceived   time.Time
	msg            interface{}
}

func (t *syncTxInflight) Context() context.Context {
	return t.ctx
}

func (t *syncTxInflight) Headers() *messages.CommonHeaders {
	query, ok := t.msg.(*messages.QueryChaincode)
	if !ok {
		byId, ok := t.msg.(*messages.GetTxById)
		if !ok {
			return &t.msg.(*messages.SendTransaction).Headers.CommonHeaders
		} else {
			return &byId.Headers.CommonHeaders
		}
	}
	return &query.Headers.CommonHeaders
}

func (t *syncTxInflight) Unmarshal(msg interface{}) error {
	reflect.ValueOf(msg).Elem().Set(reflect.ValueOf(t.msg).Elem())
	return nil
}

func (t *syncTxInflight) SendErrorReply(status int, err error) {
	t.replyProcessor.ReplyWithError(err)
}

func (t *syncTxInflight) SendErrorReplyWithTX(status int, err error, txHash string) {
	t.SendErrorReply(status, errors.Errorf(errors.RESTGatewaySyncWrapErrorWithTXDetail, txHash, err))
}

func (t *syncTxInflight) Reply(replyMessage messages.ReplyWithHeaders) {
	headers := t.Headers()
	replyHeaders := replyMessage.ReplyHeaders()
	replyHeaders.ID = utils.UUIDv4()
	replyHeaders.Context = headers.Context
	replyHeaders.ReqID = headers.ID
	replyHeaders.Received = t.timeReceived.UTC().Format(time.RFC3339Nano)
	replyTime := time.Now().UTC()
	replyHeaders.Elapsed = replyTime.Sub(t.timeReceived).Seconds()
	t.replyProcessor.ReplyWithReceipt(replyMessage)
}

func (t *syncTxInflight) String() string {
	headers := t.Headers()
	return fmt.Sprintf("MsgContext[%s/%s]", headers.MsgType, headers.ID)
}

type restReceiptAndError struct {
	Message string `json:"error"`
	messages.ReplyWithHeaders
}

type syncResponder struct {
	res    http.ResponseWriter
	req    *http.Request
	done   bool
	waiter *sync.Cond
}

func (i *syncResponder) ReplyWithError(err error) {
	errors.RestErrReply(i.res, i.req, err, 500)
	i.done = true
	i.waiter.Broadcast()
}

func (i *syncResponder) ReplyWithReceiptAndError(receipt messages.ReplyWithHeaders, err error) {
	status := 500
	reply, _ := json.MarshalIndent(&restReceiptAndError{err.Error(), receipt}, "", "  ")
	log.Infof("<-- %s %s [%d]", i.req.Method, i.req.URL, status)
	log.Debugf("<-- %s", reply)
	i.res.Header().Set("Content-Type", "application/json")
	i.res.WriteHeader(status)
	_, _ = i.res.Write(reply)
	i.done = true
	i.waiter.Broadcast()
}

func (i *syncResponder) ReplyWithReceipt(receipt messages.ReplyWithHeaders) {
	status := 200
	if receipt.ReplyHeaders().MsgType != messages.MsgTypeTransactionSuccess && receipt.ReplyHeaders().MsgType != messages.MsgTypeQuerySuccess {
		status = 500
	}
	reply, _ := json.MarshalIndent(receipt, "", "  ")
	log.Infof("<-- %s %s [%d]", i.req.Method, i.req.URL, status)
	log.Debugf("<-- %s", reply)
	i.res.Header().Set("Content-Type", "application/json")
	i.res.WriteHeader(status)
	_, _ = i.res.Write(reply)
	i.done = true
	i.waiter.Broadcast()
}

// handles transactions that require tracking transaction results
func (d *syncDispatcher) DispatchMsgSync(ctx context.Context, res http.ResponseWriter, req *http.Request, msg interface{}) {
	responder := &syncResponder{
		res:    res,
		req:    req,
		done:   false,
		waiter: sync.NewCond(&sync.Mutex{}),
	}
	syncCtx := &syncTxInflight{
		replyProcessor: responder,
		timeReceived:   time.Now().UTC(),
		msg:            msg,
		ctx:            ctx,
	}
	d.processor.OnMessage(syncCtx)
	responder.waiter.L.Lock()
	for !responder.done {
		responder.waiter.Wait()
	}
}

//
// synchronous request to Fabric API endpoints
//

func (d *syncDispatcher) QueryChaincode(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	start := time.Now().UTC()
	msg, err := restutil.BuildQueryMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}

	result, err1 := d.processor.GetRPCClient().Query(msg.Headers.ChannelID, msg.Headers.Signer, msg.Headers.ChaincodeName, msg.Function, msg.Args, msg.StrongRead)
	callTime := time.Now().UTC().Sub(start)
	if err1 != nil {
		log.Warnf("Query [chaincode=%s, func=%s] failed to send: %s [%.2fs]", msg.Headers.ChaincodeName, msg.Function, err1, callTime.Seconds())
		errors.RestErrReply(res, req, err1, 500)
		return
	}
	log.Infof("Query [chaincode=%s, func=%s] [%.2fs]", msg.Headers.ChaincodeName, msg.Function, callTime.Seconds())
	var reply messages.QueryResult
	reply.Headers.ChannelID = msg.Headers.ChannelID
	reply.Headers.ID = msg.Headers.ID
	reply.Result = utils.DecodePayload(result)
	sendReply(res, req, reply)
}

func (d *syncDispatcher) GetTxById(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	start := time.Now().UTC()
	msg, err := restutil.BuildTxByIdMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}

	result, err1 := d.processor.GetRPCClient().QueryTransaction(msg.Headers.ChannelID, msg.Headers.Signer, msg.TxId)
	callTime := time.Now().UTC().Sub(start)
	if err1 != nil {
		log.Warnf("Query transaction %s failed to send: %s [%.2fs]", msg.TxId, err1, callTime.Seconds())
		errors.RestErrReply(res, req, err1, 500)
		return
	}
	log.Infof("Query transaction %s [%.2fs]", msg.TxId, callTime.Seconds())
	var reply messages.LedgerQueryResult
	reply.Result = result

	sendReply(res, req, reply)
}

func (d *syncDispatcher) GetChainInfo(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	msg, err := restutil.BuildGetChainInfoMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}

	result, err1 := d.processor.GetRPCClient().QueryChainInfo(msg.Headers.ChannelID, msg.Headers.Signer)
	if err1 != nil {
		errors.RestErrReply(res, req, err1, 500)
		return
	}
	var reply messages.LedgerQueryResult
	m := make(map[string]interface{})
	m["height"] = result.BCI.Height
	m["current_block_hash"] = hex.EncodeToString(result.BCI.CurrentBlockHash)
	m["previous_block_hash"] = hex.EncodeToString(result.BCI.PreviousBlockHash)
	reply.Result = m

	sendReply(res, req, reply)
}

func (d *syncDispatcher) GetBlock(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	msg, err := restutil.BuildGetBlockMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}

	rawblock, block, err1 := d.processor.GetRPCClient().QueryBlock(msg.Headers.ChannelID, msg.Headers.Signer, msg.BlockNumber, msg.BlockHash)
	if err1 != nil {
		errors.RestErrReply(res, req, err1, 500)
		return
	}
	var reply messages.LedgerQueryResult
	result := make(map[string]interface{})
	result["raw"] = rawblock
	result["block"] = block
	reply.Result = result

	sendReply(res, req, reply)
}

func (d *syncDispatcher) GetBlockByTxId(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	msg, err := restutil.BuildGetBlockByTxIdMessage(res, req, params)
	if err != nil {
		errors.RestErrReply(res, req, err.Error, err.StatusCode)
		return
	}

	rawblock, block, err1 := d.processor.GetRPCClient().QueryBlockByTxId(msg.Headers.ChannelID, msg.Headers.Signer, msg.TxId)
	if err1 != nil {
		errors.RestErrReply(res, req, err1, 500)
		return
	}
	var reply messages.LedgerQueryResult
	result := make(map[string]interface{})
	result["raw"] = rawblock
	result["block"] = block
	reply.Result = result

	sendReply(res, req, reply)
}

func sendReply(res http.ResponseWriter, req *http.Request, content interface{}) {
	reply, _ := json.MarshalIndent(content, "", "  ")
	log.Infof("<-- %s %s [%d]", req.Method, req.URL, 200)
	log.Debugf("<-- %s", reply)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(200)
	_, _ = res.Write(reply)
}
