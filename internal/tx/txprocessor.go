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

package tx

import (
	"encoding/json"
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

// TxProcessor interface is called for each message, as is responsible
// for tracking all in-flight messages
type TxProcessor interface {
	OnMessage(TxContext)
	Init(client.RPCClient)
}

var highestID = 1000000

type inflightTx struct {
	id               int
	signer           string
	initialWaitDelay time.Duration
	txContext        TxContext
	tx               *fabric.Tx
	wg               sync.WaitGroup
	rpc              client.RPCClient
}

func (i *inflightTx) String() string {
	txHash := ""
	if i.tx != nil {
		txHash = i.tx.Hash
	}
	return fmt.Sprintf("TX=%s CTX=%s", txHash, i.txContext.String())
}

type txProcessor struct {
	maxTXWaitTime     time.Duration
	inflightTxsLock   *sync.Mutex
	inflightTxs       []*inflightTx
	inflightTxDelayer TxDelayTracker
	rpc               client.RPCClient
	config            *conf.RESTGatewayConf
	concurrencySlots  chan bool
}

// NewTxnProcessor constructor for message procss
func NewTxProcessor(conf *conf.RESTGatewayConf) TxProcessor {
	if conf.SendConcurrency == 0 {
		conf.SendConcurrency = defaultSendConcurrency
	}
	p := &txProcessor{
		inflightTxsLock:   &sync.Mutex{},
		inflightTxs:       []*inflightTx{},
		inflightTxDelayer: NewTxDelayTracker(),
		config:            conf,
		concurrencySlots:  make(chan bool, conf.SendConcurrency),
	}
	return p
}

func (p *txProcessor) Init(rpc client.RPCClient) {
	p.rpc = rpc
	p.maxTXWaitTime = time.Duration(p.config.MaxTXWaitTime) * time.Second
}

// OnMessage checks the type and dispatches to the correct logic
// ** From this point on the processor MUST ensure Reply is called
//    on txnContext eventually in all scenarios.
//    It cannot return an error synchronously from this function **
func (p *txProcessor) OnMessage(txContext TxContext) {

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
	case messages.MsgTypeQueryChaincode:
		var queryChaincodeMsg messages.QueryChaincode
		if unmarshalErr = txContext.Unmarshal(&queryChaincodeMsg); unmarshalErr != nil {
			break
		}
		p.OnQueryChaincodeMessage(txContext, &queryChaincodeMsg)
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
func (p *txProcessor) addInflightWrapper(txContext TxContext, msg *messages.RequestCommon) (inflight *inflightTx, err error) {

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
	inflight.initialWaitDelay = p.inflightTxDelayer.GetInitialDelay() // Must call under lock

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

// waitForCompletion is the goroutine to track a transaction through
// to completion and send the result
func (p *txProcessor) waitForCompletion(inflight *inflightTx, initialWaitDelay time.Duration) {

	// The initial delay is passed in, based on updates from all the other
	// go routines that are tracking transactions. The idea is to minimize
	// both latency beyond the block period, and avoiding spamming the node
	// with REST calls for long block periods, or when there is a backlog
	replyWaitStart := time.Now().UTC()
	time.Sleep(initialWaitDelay)

	var isMined, timedOut bool
	var err error
	var retries int
	var elapsed time.Duration
	for !isMined && !timedOut {

		if isMined, err = inflight.tx.GetTXReceipt(inflight.txContext.Context(), p.rpc); err != nil {
			// We wait even on connectivity errors, as we've submitted the transaction and
			// we want to provide a receipt if connectivity resumes within the timeout
			log.Infof("Failed to get receipt for %s (retries=%d): %s", inflight, retries, err)
		}

		elapsed = time.Now().UTC().Sub(replyWaitStart)
		timedOut = elapsed > p.maxTXWaitTime
		if !isMined && !timedOut {
			// Need to have the inflight lock to calculate the delay, but not
			// while we're waiting
			p.inflightTxsLock.Lock()
			delayBeforeRetry := p.inflightTxDelayer.GetRetryDelay(initialWaitDelay, retries+1)
			p.inflightTxsLock.Unlock()

			log.Debugf("Receipt not available after %.2fs (retries=%d): %s", elapsed.Seconds(), retries, inflight)
			time.Sleep(delayBeforeRetry)
			retries++
		}
	}

	if timedOut {
		if err != nil {
			inflight.txContext.SendErrorReplyWithTX(500, errors.Errorf(errors.TransactionSendReceiptCheckError, retries, err), inflight.tx.Hash)
		} else {
			inflight.txContext.SendErrorReplyWithTX(408, errors.Errorf(errors.TransactionSendReceiptCheckTimeout), inflight.tx.Hash)
		}
	} else {
		// Update the stats
		p.inflightTxsLock.Lock()
		p.inflightTxDelayer.ReportSuccess(elapsed)
		p.inflightTxsLock.Unlock()

		receipt := inflight.tx.Receipt
		isSuccess := receipt.IsSuccess()
		log.Infof("Receipt for %s obtained after %.2fs Success=%t", inflight.tx.Receipt.TransactionID, elapsed.Seconds(), isSuccess)

		// Build our reply
		var reply messages.TransactionReceipt
		if isSuccess {
			reply.Headers.MsgType = messages.MsgTypeTransactionSuccess
		} else {
			reply.Headers.MsgType = messages.MsgTypeTransactionFailure
		}

		inflight.txContext.Reply(&reply)
	}

	// We've submitted the transaction, even if we didn't get a receipt within our timeout.
	p.cancelInFlight(inflight, true)
	inflight.wg.Done()
}

// addInflight adds a transaction to the inflight list, and kick off
// a goroutine to check for its completion and send the result
func (p *txProcessor) trackMining(inflight *inflightTx, tx *fabric.Tx) {

	// Kick off the goroutine to track it to completion
	inflight.tx = tx
	inflight.wg.Add(1)
	go p.waitForCompletion(inflight, inflight.initialWaitDelay)

}

// func (p *txProcessor) OnDeployContractMessage(txnContext TxContext, msg *messages.DeployContract) {

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

func (p *txProcessor) OnQueryChaincodeMessage(txContext TxContext, msg *messages.QueryChaincode) {

	query := fabric.NewQuery(msg, txContext.Headers().Signer)
	result, err := query.Send(txContext.Context(), p.rpc)
	if err != nil {
		txContext.SendErrorReply(500, err)
		return
	}
	var reply messages.QueryResult
	reply.Headers.MsgType = messages.MsgTypeQuerySuccess
	// first attempt to parse for JSON, if not successful then just decode to string
	var structuredArray []map[string]interface{}
	err = json.Unmarshal(result, &structuredArray)
	if err != nil {
		structuredMap := make(map[string]interface{})
		err = json.Unmarshal(result, &structuredMap)
		if err != nil {
			reply.Result = string(result)
		} else {
			reply.Result = structuredMap
		}
	} else {
		reply.Result = structuredArray
	}

	txContext.Reply(&reply)
}

func (p *txProcessor) OnSendTransactionMessage(txContext TxContext, msg *messages.SendTransaction) {

	inflight, err := p.addInflightWrapper(txContext, &msg.RequestCommon)
	if err != nil {
		txContext.SendErrorReply(400, err)
		return
	}

	tx := fabric.NewSendTx(msg, inflight.signer)
	p.sendTransactionCommon(txContext, inflight, tx)
}

func (p *txProcessor) sendTransactionCommon(txContext TxContext, inflight *inflightTx, tx *fabric.Tx) {
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

func (p *txProcessor) sendAndTrackMining(txContext TxContext, inflight *inflightTx, tx *fabric.Tx) {
	err := tx.Send(txContext.Context(), inflight.rpc)
	if p.config.SendConcurrency > 1 {
		<-p.concurrencySlots // return our slot as soon as send is complete, to let an awaiting send go
	}
	if err != nil {
		p.cancelInFlight(inflight, false /* not confirmed as submitted, as send failed */)
		txContext.SendErrorReply(500, err)
		return
	}

	p.trackMining(inflight, tx)
}
