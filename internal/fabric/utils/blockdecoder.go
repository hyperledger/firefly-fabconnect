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

package utils

import (
	"encoding/hex"

	"github.com/golang/protobuf/proto" //nolint
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func GetEvents(block *common.Block) []*api.EventEntry {
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
						ChaincodeId:   event.ChaincodeId,
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

func DecodeBlock(block *common.Block) ([]map[string]interface{}, error) {
	// this array in the block header's metadata contains each transaction's status code
	txFilter := []byte(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])

	result := make([]map[string]interface{}, len(block.Data.Data))
	for i, data := range block.Data.Data {
		env, err := getEnvelopeFromBlock(data)
		if err != nil {
			return nil, errors.Wrap(err, "error decoding Envelope from block")
		}
		if env == nil {
			return nil, errors.New("nil envelope")
		}

		tx, err := DecodeBlockDataEnvelope(env)
		if err != nil {
			return nil, errors.Wrap(err, "error decoding block data envelope")
		}
		entry := make(map[string]interface{})
		entry["validationCode"] = peer.TxValidationCode(txFilter[i])
		entry["transaction"] = tx
	}

	return result, nil
}

func toFilteredBlock(block *common.Block) *peer.FilteredBlock {
	var channelID string
	var filteredTxs []*peer.FilteredTransaction
	// this array in the block header's metadata contains each transaction's status code
	txFilter := []byte(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])

	for i, data := range block.Data.Data {
		filteredTx, chID, err := GetFilteredTxFromBlockData(data, peer.TxValidationCode(txFilter[i]))
		if err != nil {
			log.Warnf("error decoding Envelope from block: %s", err)
			continue
		}
		channelID = chID
		filteredTxs = append(filteredTxs, filteredTx)
	}

	return &peer.FilteredBlock{
		ChannelId:            channelID,
		Number:               block.Header.Number,
		FilteredTransactions: filteredTxs,
	}
}

func GetFilteredTxFromBlockData(data []byte, txValidationCode peer.TxValidationCode) (*peer.FilteredTransaction, string, error) {
	env, err := getEnvelopeFromBlock(data)
	if err != nil {
		return nil, "", errors.Wrap(err, "error decoding Envelope from block")
	}
	if env == nil {
		return nil, "", errors.New("nil envelope")
	}
	return GetFilteredTxFromEnvelope(env, txValidationCode)
}

func DecodeBlockDataEnvelope(env *common.Envelope) (map[string]interface{}, error) {
	payload, err := UnmarshalPayload(env.Payload)
	if err != nil {
		return nil, errors.Wrap(err, "error decoding Payload from envelope")
	}

	result := make(map[string]interface{})

	channelHeader := &common.ChannelHeader{}
	if err := proto.Unmarshal(payload.Header.ChannelHeader, channelHeader); err != nil {
		return nil, errors.Wrap(err, "error decoding ChannelHeader from payload")
	}

	// TODO: support other block types (1=ConfigEvelope, 2=ConfigUpdateEnvelope)
	if channelHeader.Type == 3 {
		result["type"] = "EndorserTransaction"

		tx := &peer.Transaction{}
		if err := proto.Unmarshal(payload.Data, tx); err != nil {
			return nil, errors.Wrap(err, "error decoding transaction payload data")
		}

		transactions := make([]map[string]interface{}, len(tx.Actions))
		result["transactions"] = transactions

		for i, action := range tx.Actions {
			transaction := make(map[string]interface{})
			transactions[i] = transaction

			chaincodeActionPayload := make(map[string]interface{})
			transaction["payload"] = chaincodeActionPayload
			cap := &peer.ChaincodeActionPayload{}
			if err := proto.Unmarshal(action.Payload, cap); err != nil {
				return nil, errors.Wrap(err, "error decoding chaincode action payload")
			}
			// proposal payload part of the chaincode action
			chaincodeProposalPayload := make(map[string]interface{})
			chaincodeActionPayload["chaincodeProposalPayload"] = chaincodeProposalPayload
			cpp := &peer.ChaincodeProposalPayload{}
			if err = proto.Unmarshal(cap.ChaincodeProposalPayload, cpp); err != nil {
				return nil, errors.Wrap(err, "error decoding chaincode proposal payload")
			}
			chaincodeInvocationSpec := make(map[string]interface{})
			chaincodeProposalPayload["input"] = chaincodeInvocationSpec
			ccSpec := &peer.ChaincodeInvocationSpec{}
			if err := proto.Unmarshal(cpp.Input, ccSpec); err != nil {
				return nil, errors.Wrap(err, "error decoding chaincode invocation spec")
			}

			chaincodeInvocationSpec["type"] = ccSpec.ChaincodeSpec.Type
			chaincodeInvocationSpec["chaincodeId"] = ccSpec.ChaincodeSpec.ChaincodeId

			chaincodeInput := make(map[string]interface{})
			chaincodeInputArgs := make([]string, len(ccSpec.ChaincodeSpec.Input.Args))
			for j, arg := range ccSpec.ChaincodeSpec.Input.Args {
				chaincodeInputArgs[j] = string(arg)
			}
			chaincodeInput["args"] = chaincodeInputArgs
			chaincodeInput["isInit"] = ccSpec.ChaincodeSpec.Input.IsInit
			chaincodeInvocationSpec["input"] = chaincodeInput

			chaincodeProposal := make(map[string]interface{})
			chaincodeProposal["input"] = chaincodeInput
			transaction["chaincodeProposal"] = chaincodeProposal

			// endorsed action part of the chaincode action
			chaincodeEndorsedAction := make(map[string]interface{})
			chaincodeActionPayload["action"] = chaincodeEndorsedAction

			proposalResponsePayload := make(map[string]interface{})
			chaincodeEndorsedAction["proposalResponsePayload"] = proposalResponsePayload
			prp := &peer.ProposalResponsePayload{}
			if err = proto.Unmarshal(cap.Action.ProposalResponsePayload, prp); err != nil {
				return nil, errors.Wrap(err, "error decoding chaincode proposal response payload")
			}

			proposalResponsePayload["proposalHash"] = hex.EncodeToString(prp.ProposalHash)

			extension := make(map[string]interface{})
			proposalResponsePayload["extension"] = extension
			cca := &peer.ChaincodeAction{}
			if err = proto.Unmarshal(prp.Extension, cca); err != nil {
				return nil, errors.Wrap(err, "error decoding chaincode action")
			}

			// skipping read/write sets in the action
			// decode events
			chaincodeEvent := make(map[string]interface{})
			extension["events"] = chaincodeEvent
			ccevt := &peer.ChaincodeEvent{}
			if err = proto.Unmarshal(cca.Events, ccevt); err != nil {
				return nil, errors.Wrap(err, "error decoding chaincode event")
			}
			chaincodeEvent["chaincodeId"] = ccevt.ChaincodeId
			chaincodeEvent["txId"] = ccevt.TxId
			chaincodeEvent["eventName"] = ccevt.EventName
			chaincodeEvent["payload"] = string(ccevt.Payload)
		}
	}

	return result, nil
}

func GetFilteredTxFromEnvelope(env *common.Envelope, txValidationCode peer.TxValidationCode) (*peer.FilteredTransaction, string, error) {
	payload, err := UnmarshalPayload(env.Payload)
	if err != nil {
		return nil, "", errors.Wrap(err, "error decoding Payload from envelope")
	}

	channelHeaderBytes := payload.Header.ChannelHeader
	channelHeader := &common.ChannelHeader{}
	if err := proto.Unmarshal(channelHeaderBytes, channelHeader); err != nil {
		return nil, "", errors.Wrap(err, "error decoding ChannelHeader from payload")
	}

	filteredTx := &peer.FilteredTransaction{
		Type:             common.HeaderType(channelHeader.Type),
		Txid:             channelHeader.TxId,
		TxValidationCode: txValidationCode,
	}

	if common.HeaderType(channelHeader.Type) == common.HeaderType_ENDORSER_TRANSACTION {
		actions, err := getFilteredTransactionActions(payload.Data)
		if err != nil {
			return nil, "", errors.Wrap(err, "error getting filtered transaction actions")
		}
		filteredTx.Data = actions
	}
	return filteredTx, channelHeader.ChannelId, nil
}

func getFilteredTransactionActions(data []byte) (*peer.FilteredTransaction_TransactionActions, error) {
	actions := &peer.FilteredTransaction_TransactionActions{
		TransactionActions: &peer.FilteredTransactionActions{},
	}
	tx, err := UnmarshalTransaction(data)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling transaction payload")
	}
	chaincodeActionPayload, err := UnmarshalChaincodeActionPayload(tx.Actions[0].Payload)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling chaincode action payload")
	}
	propRespPayload, err := UnmarshalProposalResponsePayload(chaincodeActionPayload.Action.ProposalResponsePayload)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling response payload")
	}
	ccAction, err := UnmarshalChaincodeAction(propRespPayload.Extension)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling chaincode action")
	}
	ccEvent, err := UnmarshalChaincodeEvents(ccAction.Events)
	if err != nil {
		return nil, errors.Wrap(err, "error getting chaincode events")
	}
	if ccEvent != nil {
		actions.TransactionActions.ChaincodeActions = append(actions.TransactionActions.ChaincodeActions, &peer.FilteredChaincodeAction{ChaincodeEvent: ccEvent})
	}
	return actions, nil
}

func getEnvelopeFromBlock(data []byte) (*common.Envelope, error) {
	// Block always begins with an envelope
	var err error
	env := &common.Envelope{}
	if err = proto.Unmarshal(data, env); err != nil {
		return nil, errors.Wrap(err, "error unmarshaling Envelope")
	}

	return env, nil
}

// UnmarshalPayload unmarshals bytes to a Payload
func UnmarshalPayload(encoded []byte) (*common.Payload, error) {
	payload := &common.Payload{}
	err := proto.Unmarshal(encoded, payload)
	return payload, errors.Wrap(err, "error unmarshaling Payload")
}

// UnmarshalTransaction unmarshals bytes to a Transaction
func UnmarshalTransaction(txBytes []byte) (*peer.Transaction, error) {
	tx := &peer.Transaction{}
	err := proto.Unmarshal(txBytes, tx)
	return tx, errors.Wrap(err, "error unmarshaling Transaction")
}

// UnmarshalChaincodeActionPayload unmarshals bytes to a ChaincodeActionPayload
func UnmarshalChaincodeActionPayload(capBytes []byte) (*peer.ChaincodeActionPayload, error) {
	cap := &peer.ChaincodeActionPayload{}
	err := proto.Unmarshal(capBytes, cap)
	return cap, errors.Wrap(err, "error unmarshaling ChaincodeActionPayload")
}

// UnmarshalProposalResponsePayload unmarshals bytes to a ProposalResponsePayload
func UnmarshalProposalResponsePayload(prpBytes []byte) (*peer.ProposalResponsePayload, error) {
	prp := &peer.ProposalResponsePayload{}
	err := proto.Unmarshal(prpBytes, prp)
	return prp, errors.Wrap(err, "error unmarshaling ProposalResponsePayload")
}

// UnmarshalChaincodeAction unmarshals bytes to a ChaincodeAction
func UnmarshalChaincodeAction(caBytes []byte) (*peer.ChaincodeAction, error) {
	chaincodeAction := &peer.ChaincodeAction{}
	err := proto.Unmarshal(caBytes, chaincodeAction)
	return chaincodeAction, errors.Wrap(err, "error unmarshaling ChaincodeAction")
}

// UnmarshalChaincodeEvents unmarshals bytes to a ChaincodeEvent
func UnmarshalChaincodeEvents(eBytes []byte) (*peer.ChaincodeEvent, error) {
	chaincodeEvent := &peer.ChaincodeEvent{}
	err := proto.Unmarshal(eBytes, chaincodeEvent)
	return chaincodeEvent, errors.Wrap(err, "error unmarshaling ChaicnodeEvent")
}
