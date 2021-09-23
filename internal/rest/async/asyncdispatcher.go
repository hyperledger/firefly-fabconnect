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
	"net/http"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/receipt"
	"github.com/hyperledger/firefly-fabconnect/internal/tx"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// AsyncDispatcher is passed in to process messages over a streaming system with
// a receipt store. Only used for POST methods, when fly-sync is not set to true
type AsyncDispatcher interface {
	ValidateConf() error
	Run() error
	IsInitialized() bool
	DispatchMsgAsync(ctx context.Context, msg *messages.SendTransaction, ack bool) (*messages.AsyncSentMsg, error)
	HandleReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params)
	Close()
}

// Interface to be implemented by the direct handler and kafka-based handler
type asyncRequestHandler interface {
	validateHandlerConf() error
	dispatchMsg(ctx context.Context, key, msgID string, msg *messages.SendTransaction, ack bool) (msgAck string, statusCode int, err error)
	run() error
	isInitialized() bool
}

type asyncDispatcher struct {
	handler      asyncRequestHandler
	receiptStore receipt.ReceiptStore
}

func NewAsyncDispatcher(conf *conf.RESTGatewayConf, processor tx.TxProcessor, receiptstore receipt.ReceiptStore) AsyncDispatcher {
	var handler asyncRequestHandler
	if len(conf.Kafka.Brokers) > 0 {
		handler = newKafkaHandler(conf.Kafka, receiptstore)
	} else {
		handler = newDirectHandler(conf, processor, receiptstore)
	}

	return &asyncDispatcher{
		handler:      handler,
		receiptStore: receiptstore,
	}
}

func (d *asyncDispatcher) ValidateConf() error {
	err := d.handler.validateHandlerConf()
	if err != nil {
		return err
	}
	err = d.receiptStore.ValidateConf()
	return err
}

// DispatchMsgAsync is the interface method for async dispatching of messages
func (d *asyncDispatcher) DispatchMsgAsync(ctx context.Context, msg *messages.SendTransaction, ack bool) (*messages.AsyncSentMsg, error) {
	reply, _, err := d.processMsg(ctx, msg, ack)
	return reply, err
}

func (d *asyncDispatcher) HandleReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	p := req.URL.Path
	if p == "/receipts" {
		d.receiptStore.GetReceipts(res, req, params)
	} else {
		d.receiptStore.GetReceipt(res, req, params)
	}
}

func (d *asyncDispatcher) Close() {
	d.receiptStore.Close()
}

func (w *asyncDispatcher) processMsg(ctx context.Context, msg *messages.SendTransaction, ack bool) (*messages.AsyncSentMsg, int, error) {
	switch msg.Headers.MsgType {
	case messages.MsgTypeDeployContract, messages.MsgTypeSendTransaction:
		if msg.Headers.Signer == "" {
			return nil, 400, errors.Errorf(errors.RequestHandlerInvalidMsgSignerMissing)
		}
	default:
		return nil, 400, errors.Errorf(errors.RequestHandlerInvalidMsgType, msg.Headers.MsgType)
	}

	// Generate a message ID if not already set
	if msg.Headers.ID == "" {
		msgID := utils.UUIDv4()
		msg.Headers.ID = msgID
	}

	// Pass to the handler
	log.Infof("Request handler accepted message. MsgID: %s Type: %s", msg.Headers.ID, msg.Headers.MsgType)
	msgAck, status, err := w.handler.dispatchMsg(ctx, msg.Headers.ChannelID, msg.Headers.ID, msg, ack)
	if err != nil {
		return nil, status, err
	}
	return &messages.AsyncSentMsg{
		Sent:    true,
		Request: msg.Headers.ID,
		Msg:     msgAck,
	}, 200, nil
}

func (w *asyncDispatcher) Run() error {
	return w.handler.run()
}

func (w *asyncDispatcher) IsInitialized() bool {
	return w.handler.isInitialized()
}
