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

package fabric

import (
	"github.com/golang/protobuf/proto" //nolint
	"github.com/hyperledger-labs/firefly-fabconnect/internal/events/api"
	cb "github.com/hyperledger/fabric-protos-go/common"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func GetEvents(block *cb.Block) []*api.EventEntry {
	events := []*api.EventEntry{}
	fb := toFilteredBlock(block)
	for _, tx := range fb.GetFilteredTransactions() {
		actions := tx.GetTransactionActions()
		if actions != nil {
			ccActions := actions.GetChaincodeActions()
			if len(ccActions) > 0 {
				for _, ccAction := range ccActions {
					event := ccAction.GetChaincodeEvent()
					eventEntry := api.EventEntry{
						ChaincodeId:   event.TxId,
						BlockNumber:   fb.Number,
						TransactionId: tx.Txid,
						EventName:     event.EventName,
						Payload:       event.Payload,
					}
					events = append(events, &eventEntry)
				}
			}
		}
	}
	return events
}

func toFilteredBlock(block *cb.Block) *pb.FilteredBlock {
	var channelID string
	var filteredTxs []*pb.FilteredTransaction
	// this array in the block header's metadata contains each transaction's status code
	txFilter := []byte(block.Metadata.Metadata[cb.BlockMetadataIndex_TRANSACTIONS_FILTER])

	for i, data := range block.Data.Data {
		filteredTx, chID, err := getFilteredTx(data, pb.TxValidationCode(txFilter[i]))
		if err != nil {
			log.Warnf("error extracting Envelope from block: %s", err)
			continue
		}
		channelID = chID
		filteredTxs = append(filteredTxs, filteredTx)
	}

	return &pb.FilteredBlock{
		ChannelId:            channelID,
		Number:               block.Header.Number,
		FilteredTransactions: filteredTxs,
	}
}

func getFilteredTx(data []byte, txValidationCode pb.TxValidationCode) (*pb.FilteredTransaction, string, error) {
	env, err := protoutil.GetEnvelopeFromBlock(data)
	if err != nil {
		return nil, "", errors.Wrap(err, "error extracting Envelope from block")
	}
	if env == nil {
		return nil, "", errors.New("nil envelope")
	}

	payload, err := protoutil.UnmarshalPayload(env.Payload)
	if err != nil {
		return nil, "", errors.Wrap(err, "error extracting Payload from envelope")
	}

	channelHeaderBytes := payload.Header.ChannelHeader
	channelHeader := &cb.ChannelHeader{}
	if err := proto.Unmarshal(channelHeaderBytes, channelHeader); err != nil {
		return nil, "", errors.Wrap(err, "error extracting ChannelHeader from payload")
	}

	filteredTx := &pb.FilteredTransaction{
		Type:             cb.HeaderType(channelHeader.Type),
		Txid:             channelHeader.TxId,
		TxValidationCode: txValidationCode,
	}

	if cb.HeaderType(channelHeader.Type) == cb.HeaderType_ENDORSER_TRANSACTION {
		actions, err := getFilteredTransactionActions(payload.Data)
		if err != nil {
			return nil, "", errors.Wrap(err, "error getting filtered transaction actions")
		}
		filteredTx.Data = actions
	}
	return filteredTx, channelHeader.ChannelId, nil
}

func getFilteredTransactionActions(data []byte) (*pb.FilteredTransaction_TransactionActions, error) {
	actions := &pb.FilteredTransaction_TransactionActions{
		TransactionActions: &pb.FilteredTransactionActions{},
	}
	tx, err := protoutil.UnmarshalTransaction(data)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling transaction payload")
	}
	chaincodeActionPayload, err := protoutil.UnmarshalChaincodeActionPayload(tx.Actions[0].Payload)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling chaincode action payload")
	}
	propRespPayload, err := protoutil.UnmarshalProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling response payload")
	}
	ccAction, err := protoutil.UnmarshalChaincodeAction(propRespPayload.Extension)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling chaincode action")
	}
	ccEvent, err := protoutil.UnmarshalChaincodeEvents(ccAction.Events)
	if err != nil {
		return nil, errors.Wrap(err, "error getting chaincode events")
	}
	if ccEvent != nil {
		actions.TransactionActions.ChaincodeActions = append(actions.TransactionActions.ChaincodeActions, &pb.FilteredChaincodeAction{ChaincodeEvent: ccEvent})
	}
	return actions, nil
}
