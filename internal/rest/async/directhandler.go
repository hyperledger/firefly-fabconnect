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

package async

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/receipt"
	"github.com/hyperledger/firefly-fabconnect/internal/tx"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	log "github.com/sirupsen/logrus"
)

type directHandler struct {
	initialized   bool
	receipts      receipt.ReceiptStore
	conf          *conf.RESTGatewayConf
	processor     tx.TxProcessor
	inFlightMutex sync.Mutex
	inFlight      map[string]*msgContext
	stopChan      chan error
}

func newDirectHandler(conf *conf.RESTGatewayConf, processor tx.TxProcessor, receiptstore receipt.ReceiptStore) *directHandler {
	return &directHandler{
		processor: processor,
		receipts:  receiptstore,
		conf:      conf,
		inFlight:  make(map[string]*msgContext),
		stopChan:  make(chan error),
	}
}

type msgContext struct {
	ctx          context.Context
	w            *directHandler
	timeReceived time.Time
	key          string
	msgID        string
	msg          *messages.SendTransaction
	headers      *messages.CommonHeaders
}

func (t *msgContext) Context() context.Context {
	return t.ctx
}

func (t *msgContext) Headers() *messages.CommonHeaders {
	return t.headers
}

func (t *msgContext) Unmarshal(msg interface{}) error {
	reflect.ValueOf(msg).Elem().Set(reflect.ValueOf(t.msg).Elem())
	return nil
}

func (t *msgContext) SendErrorReply(status int, err error) {
	t.SendErrorReplyWithTX(status, err, "")
}

func (t *msgContext) SendErrorReplyWithTX(status int, err error, txHash string) {
	log.Warnf("Failed to process message %s: %s", t, err)
	origBytes, _ := json.Marshal(t.msg)
	errMsg := messages.NewErrorReply(err, origBytes)
	errMsg.TXHash = txHash
	t.Reply(errMsg)
}

func (t *msgContext) Reply(replyMessage messages.ReplyWithHeaders) {
	t.w.inFlightMutex.Lock()
	defer t.w.inFlightMutex.Unlock()

	replyHeaders := replyMessage.ReplyHeaders()
	replyHeaders.ID = utils.UUIDv4()
	replyHeaders.Context = t.headers.Context
	replyHeaders.ReqID = t.headers.ID
	replyHeaders.Received = t.timeReceived.UTC().Format(time.RFC3339Nano)
	replyTime := time.Now().UTC()
	replyHeaders.Elapsed = replyTime.Sub(t.timeReceived).Seconds()
	msgBytes, _ := json.Marshal(&replyMessage)
	t.w.receipts.ProcessReceipt(msgBytes)
	delete(t.w.inFlight, t.msgID)
}

func (t *msgContext) String() string {
	return fmt.Sprintf("MsgContext[%s/%s]", t.headers.MsgType, t.msgID)
}

func (w *directHandler) dispatchMsg(ctx context.Context, key, msgID string, msg *messages.SendTransaction, ack bool) (string, int, error) {
	w.inFlightMutex.Lock()

	numInFlight := len(w.inFlight)
	if numInFlight >= w.conf.MaxInFlight {
		w.inFlightMutex.Unlock()
		log.Errorf("Failed to dispatch mesage from '%s': %d/%d already in-flight", key, numInFlight, w.conf.MaxInFlight)
		return "", 429, errors.Errorf(errors.RequestHandlerDirectTooManyInflight)
	}

	msgContext := &msgContext{
		ctx:          context.Background(),
		w:            w,
		timeReceived: time.Now().UTC(),
		key:          key,
		msgID:        msgID,
		msg:          msg,
		headers:      &msg.Headers.CommonHeaders,
	}
	w.inFlight[msgID] = msgContext
	w.inFlightMutex.Unlock()

	w.processor.OnMessage(msgContext)
	return "", 200, nil
}

func (w *directHandler) validateHandlerConf() error {
	if w.conf.MaxTXWaitTime < 10 {
		if w.conf.MaxTXWaitTime > 0 {
			log.Warnf("Maximum wait time increased from %d to minimum of 10 seconds", w.conf.MaxTXWaitTime)
		}
		w.conf.MaxTXWaitTime = 10
	}
	if w.conf.MaxInFlight <= 0 {
		w.conf.MaxInFlight = 10
	}
	return nil
}

func (w *directHandler) run() error {
	w.initialized = true
	return <-w.stopChan
}

func (w *directHandler) isInitialized() bool {
	return w.initialized
}
