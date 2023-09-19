// Copyright © 2023 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tx

import (
	"fmt"
	"sync"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/client"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	log "github.com/sirupsen/logrus"
)

const (
	defaultSendConcurrency = 1
)

// Processor interface is called for each message, as is responsible
// for tracking all in-flight messages
type Processor interface {
	OnMessage(Context)
	Init(client.RPCClient)
	GetRPCClient() client.RPCClient
}

var highestID = 1000000

type inflightTx struct {
	id        int
	signer    string
	txContext Context
	tx        *fabric.Tx
	rpc       client.RPCClient
}

func (i *inflightTx) String() string {
	txHash := ""
	if i.tx != nil {
		txHash = i.tx.Hash
	}
	return fmt.Sprintf("TX=%s CTX=%s", txHash, i.txContext.String())
}

type txProcessor struct {
	maxTXWaitTime    time.Duration
	inflightTxsLock  *sync.Mutex
	inflightTxs      []*inflightTx
	rpc              client.RPCClient
	config           *conf.RESTGatewayConf
	concurrencySlots chan bool
}

// NewTxnProcessor constructor for message procss
func NewTxProcessor(conf *conf.RESTGatewayConf) Processor {
	if conf.SendConcurrency == 0 {
		conf.SendConcurrency = defaultSendConcurrency
	}
	p := &txProcessor{
		inflightTxsLock:  &sync.Mutex{},
		inflightTxs:      []*inflightTx{},
		config:           conf,
		concurrencySlots: make(chan bool, conf.SendConcurrency),
	}
	return p
}

func (p *txProcessor) Init(rpc client.RPCClient) {
	p.rpc = rpc
	p.maxTXWaitTime = time.Duration(p.config.MaxTXWaitTime) * time.Second
}

func (p *txProcessor) GetRPCClient() client.RPCClient {
	return p.rpc
}

// OnMessage checks the type and dispatches to the correct logic
// ** From this point on the processor MUST ensure Reply is called
//
//	on txnContext eventually in all scenarios.
//	It cannot return an error synchronously from this function **
func (p *txProcessor) OnMessage(txContext Context) {

	var unmarshalErr error
	headers := txContext.Headers()
	log.Debugf("Processing %+v", headers)
	switch headers.MsgType {
	case messages.MsgTypeSendTransaction:
		var sendTransactionMsg messages.SendTransaction
		if unmarshalErr = txContext.Unmarshal(&sendTransactionMsg); unmarshalErr != nil {
			break
		}
		p.OnSendTransactionMessage(txContext, &sendTransactionMsg)
	default:
		unmarshalErr = errors.Errorf(errors.TransactionSendMsgTypeUnknown, headers.MsgType)
	}
	// We must always send a reply
	if unmarshalErr != nil {
		txContext.SendErrorReply(400, unmarshalErr)
	}

}

// newInflightWrapper uses the supplied transaction, the inflight txn list.
// Builds a new wrapper containing this information, that can be added to
// the inflight list if the transaction is submitted
func (p *txProcessor) addInflightWrapper(txContext Context, msg *messages.RequestCommon) (inflight *inflightTx, err error) {

	inflight = &inflightTx{
		txContext: txContext,
		signer:    msg.Headers.Signer,
	}

	// Use the correct RPC for sending transactions
	inflight.rpc = p.rpc

	// Hold the lock just while we're adding it to the map
	p.inflightTxsLock.Lock()

	inflight.id = highestID
	highestID++

	before := len(p.inflightTxs)
	p.inflightTxs = append(p.inflightTxs, inflight)

	// Clear lock before logging
	p.inflightTxsLock.Unlock()

	log.Infof("In-flight %d added. signer=%s before=%d", inflight.id, inflight.signer, before)

	return
}

// take the transaction off the inflight list for tracking, if error happened
// and there is no need to continue to track the transaction
func (p *txProcessor) cancelInFlight(inflight *inflightTx, submitted bool) {
	var before, after int
	p.inflightTxsLock.Lock()
	// Remove from the in-flight list
	before = len(p.inflightTxs)
	for idx, alreadyInflight := range p.inflightTxs {
		if alreadyInflight.id == inflight.id {
			p.inflightTxs = append(p.inflightTxs[0:idx], p.inflightTxs[idx+1:]...)
			break
		}
	}
	after = len(p.inflightTxs)
	p.inflightTxsLock.Unlock()

	log.Infof("In-flight %d complete. signer=%s sub=%t before=%d after=%d", inflight.id, inflight.signer, submitted, before, after)
}

func (p *txProcessor) processCompletion(inflight *inflightTx, _ *fabric.Tx) {

	receipt := inflight.tx.Receipt
	isSuccess := receipt.IsSuccess()
	log.Infof("Receipt for %s obtained Success=%t", inflight.tx.Receipt.TransactionID, isSuccess)

	// Build our reply
	var reply messages.TransactionReceipt
	if isSuccess {
		reply.Headers.MsgType = messages.MsgTypeTransactionSuccess
	} else {
		reply.Headers.MsgType = messages.MsgTypeTransactionFailure
	}

	reply.Status = receipt.Status.String()
	reply.BlockNumber = receipt.BlockNumber
	reply.Signer = receipt.Signer
	reply.SignerMSP = receipt.SignerMSP
	reply.TransactionHash = receipt.TransactionID

	inflight.txContext.Reply(&reply)

	// We've submitted the transaction, even if we didn't get a receipt within our timeout.
	p.cancelInFlight(inflight, true)

}

// func (p *txProcessor) OnDeployContractMessage(txnContext Context, msg *messages.DeployContract) {

// 	inflight, err := p.addInflightWrapper(txnContext, &msg.TransactionCommon)
// 	if err != nil {
// 		txnContext.SendErrorReply(400, err)
// 		return
// 	}
// 	inflight.registerAs = msg.RegisterAs
// 	msg.Nonce = inflight.nonceNumber()

// 	tx, err := fabric.NewContractDeployTxn(msg, inflight.signer)
// 	if err != nil {
// 		p.cancelInFlight(inflight, false /* not yet submitted */)
// 		txnContext.SendErrorReply(400, err)
// 		return
// 	}

// 	p.sendTransactionCommon(txnContext, inflight, tx)
// }

func (p *txProcessor) OnSendTransactionMessage(txContext Context, msg *messages.SendTransaction) {

	inflight, err := p.addInflightWrapper(txContext, &msg.RequestCommon)
	if err != nil {
		txContext.SendErrorReply(400, err)
		return
	}

	tx := fabric.NewSendTx(msg, inflight.signer)
	inflight.tx = tx
	p.sendTransactionCommon(txContext, inflight, tx)
}

func (p *txProcessor) sendTransactionCommon(txContext Context, inflight *inflightTx, tx *fabric.Tx) {
	if p.config.SendConcurrency > 1 {
		// The above must happen synchronously for each partition in Kafka - as it is where we assign the nonce.
		// However, the send to the node can happen at high concurrency.
		p.concurrencySlots <- true
		go p.sendAndTrackMining(txContext, inflight, tx)
	} else {
		// For the special case of 1 we do it synchronously, so we don't assign the next nonce until we've sent this one
		p.sendAndTrackMining(txContext, inflight, tx)
	}
}

func (p *txProcessor) sendAndTrackMining(txContext Context, inflight *inflightTx, tx *fabric.Tx) {
	err := tx.Send(txContext.Context(), inflight.rpc)
	if p.config.SendConcurrency > 1 {
		<-p.concurrencySlots // return our slot as soon as send is complete, to let an awaiting send go
	}
	if err != nil {
		p.cancelInFlight(inflight, false /* not confirmed as submitted, as send failed */)
		txContext.SendErrorReply(500, err)
		return
	}

	// receipt is already available once the "Send()" call returns
	p.processCompletion(inflight, tx)
}
