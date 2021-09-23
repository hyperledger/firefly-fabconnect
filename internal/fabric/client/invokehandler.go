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

package client

import (
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel/invoke"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/status"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
)

// adapted from the CommitHandler in https://github.com/hyperledger/fabric-sdk-go
// in order to custom process the transaction status event
type txSubmitAndListenHandler struct {
	txStatusEvent *fab.TxStatusEvent
}

func NewTxSubmitAndListenHandler(txStatus *fab.TxStatusEvent) *txSubmitAndListenHandler {
	return &txSubmitAndListenHandler{
		txStatusEvent: txStatus,
	}
}

func (h *txSubmitAndListenHandler) Handle(requestContext *invoke.RequestContext, clientContext *invoke.ClientContext) {
	txnID := requestContext.Response.TransactionID

	//Register Tx event
	reg, statusNotifier, err := clientContext.EventService.RegisterTxStatusEvent(string(txnID))
	if err != nil {
		requestContext.Error = errors.Errorf("Error registering for TxStatus event for transaction %s. %s", txnID, err)
		return
	}
	defer clientContext.EventService.Unregister(reg)

	_, err = createAndSendTransaction(clientContext.Transactor, requestContext.Response.Proposal, requestContext.Response.Responses)
	if err != nil {
		requestContext.Error = errors.Errorf("CreateAndSendTransaction failed. %s", err)
		return
	}

	select {
	case txStatus := <-statusNotifier:
		requestContext.Response.TxValidationCode = txStatus.TxValidationCode
		h.txStatusEvent.BlockNumber = txStatus.BlockNumber
		h.txStatusEvent.SourceURL = txStatus.SourceURL
		h.txStatusEvent.TxID = txStatus.TxID
		h.txStatusEvent.TxValidationCode = txStatus.TxValidationCode

		if txStatus.TxValidationCode != pb.TxValidationCode_VALID {
			requestContext.Error = status.New(status.EventServerStatus, int32(txStatus.TxValidationCode),
				"received invalid transaction", nil)
			return
		}
	case <-requestContext.Ctx.Done():
		requestContext.Error = status.New(status.ClientStatus, status.Timeout.ToInt32(),
			"Execute didn't receive block event", nil)
		return
	}
}

func createAndSendTransaction(sender fab.Sender, proposal *fab.TransactionProposal, resps []*fab.TransactionProposalResponse) (*fab.TransactionResponse, error) {

	txnRequest := fab.TransactionRequest{
		Proposal:          proposal,
		ProposalResponses: resps,
	}

	tx, err := sender.CreateTransaction(txnRequest)
	if err != nil {
		return nil, errors.Errorf("Create Transaction failed: %s", err)
	}

	transactionResponse, err := sender.SendTransaction(tx)
	if err != nil {
		return nil, errors.Errorf("Send Transaction failed: %s", err)

	}

	return transactionResponse, nil
}
