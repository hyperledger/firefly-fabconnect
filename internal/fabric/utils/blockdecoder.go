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
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto" //nolint
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/pkg/errors"
)

func GetEvents(block *common.Block) []*api.EventEntry {
	events := []*api.EventEntry{}
	rawBlock, _, err := DecodeBlock(block)
	if err != nil {
		return events
	}
	for idx, entry := range rawBlock.Data.Data {
		timestamp := entry.Payload.Header.ChannelHeader.Timestamp
		txId := entry.Payload.Header.ChannelHeader.TxId
		actions := entry.Payload.Data.Actions
		for actionIdx, action := range actions {
			event := action.Payload.Action.ProposalResponsePayload.Extension.Events
			if event == nil {
				continue
			}
			// if the transaction did not publish any events, we need to get most of the
			// information below from other parts of the transaction
			chaincodeId := action.Payload.Action.ProposalResponsePayload.Extension.ChaincodeId.Name
			eventEntry := api.EventEntry{
				ChaincodeId:      chaincodeId,
				BlockNumber:      rawBlock.Header.Number,
				TransactionId:    txId,
				TransactionIndex: idx,
				EventIndex:       actionIdx, // each action only allowed one event, so event index is the action index
				EventName:        event.EventName,
				Payload:          event.Payload,
				Timestamp:        timestamp,
			}
			events = append(events, &eventEntry)
		}
	}
	return events
}

func DecodeBlock(block *common.Block) (*RawBlock, *Block, error) {
	rawblock := &RawBlock{}
	rawblock.Header = block.Header
	rawblock.Metadata = block.Metadata
	blockdata := &BlockData{}
	rawblock.Data = blockdata
	dataEnvs := make([]*BlockDataEnvelope, len(block.Data.Data))
	blockdata.Data = dataEnvs

	bloc := &Block{}

	// this array in the block header's metadata contains each transaction's status code
	txFilter := []byte(block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER])
	var transactions []*Transaction
	for i, data := range block.Data.Data {
		env, err := getEnvelopeFromBlock(data)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error decoding Envelope from block")
		}
		if env == nil {
			return nil, nil, errors.New("nil envelope")
		}

		data, payload, err := rawblock.DecodeBlockDataEnvelope(env)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error decoding block data envelope")
		}
		dataEnvs[i] = data
		tx, ok := payload.(*Transaction)
		if ok {
			if transactions == nil {
				transactions = make([]*Transaction, len(block.Data.Data))
				bloc.Transactions = transactions
			}
			transactions[i] = tx
			tx.Status = peer.TxValidationCode(txFilter[i]).String()
		} else {
			// there should only be one config record in the block data
			config := payload.(*ConfigRecord)
			bloc.Config = config
		}
	}

	bloc.Number = rawblock.Header.Number
	bloc.DataHash = hex.EncodeToString(rawblock.Header.DataHash)
	bloc.PreviousHash = hex.EncodeToString(rawblock.Header.PreviousHash)

	return rawblock, bloc, nil
}

func (block *RawBlock) DecodeBlockDataEnvelope(env *common.Envelope) (*BlockDataEnvelope, interface{}, error) {
	// used for the raw block
	dataEnv := &BlockDataEnvelope{}
	dataEnv.Signature = base64.StdEncoding.EncodeToString(env.Signature)

	_payload := &Payload{}
	dataEnv.Payload = _payload

	payload, err := UnmarshalPayload(env.Payload)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error decoding Payload from envelope")
	}

	result, err := block.decodePayload(payload, _payload)
	if err != nil {
		return nil, nil, err
	}
	tx, ok := result.(*Transaction)
	if ok {
		tx.Signature = dataEnv.Signature
		return dataEnv, tx, nil
	} else {
		config := result.(*ConfigRecord)
		config.Signature = dataEnv.Signature
		return dataEnv, config, nil
	}
}

// based on the channel header type, returns a *Transaction (type=1) or a *ConfigRecord (type=3)
func (block *RawBlock) decodePayload(payload *common.Payload, _payload *Payload) (interface{}, error) {
	_payloadData := &PayloadData{}
	_payload.Data = _payloadData
	_payloadHeader := &PayloadHeader{}
	_payload.Header = _payloadHeader

	if err := block.decodePayloadHeader(payload.Header, _payloadHeader); err != nil {
		return nil, err
	}

	timestamp := _payloadHeader.ChannelHeader.Timestamp
	creator := _payloadHeader.SignatureHeader.Creator
	_creator := &Creator{
		MspID: creator.Mspid,
		Cert:  string(creator.IdBytes),
	}

	if _payloadHeader.ChannelHeader.Type == common.HeaderType_name[3] {
		// used in the user-friendly block
		_transaction := &Transaction{}
		_transaction.Type = _payloadHeader.ChannelHeader.Type
		_transaction.Timestamp = timestamp
		_transaction.Creator = _creator
		_transaction.Nonce = hex.EncodeToString([]byte(_payloadHeader.SignatureHeader.Nonce))
		_transaction.TxId = _payloadHeader.ChannelHeader.TxId

		if err := block.decodeTxPayloadData(payload.Data, _payloadData, _transaction); err != nil {
			return nil, err
		}
		for _, action := range _payloadData.Actions {
			if action.Payload.Action.ProposalResponsePayload.Extension.Events != nil {
				action.Payload.Action.ProposalResponsePayload.Extension.Events.Timestamp = strconv.FormatInt(timestamp, 10)
			}
		}
		return _transaction, nil
	} else if _payloadHeader.ChannelHeader.Type == common.HeaderType_name[1] {
		_configRec := &ConfigRecord{}
		_configRec.Type = _payloadHeader.ChannelHeader.Type
		_configRec.Timestamp = timestamp
		_configRec.Creator = _creator
		_configRec.Nonce = hex.EncodeToString([]byte(_payloadHeader.SignatureHeader.Nonce))

		if err := block.decodeConfigPayloadData(payload.Data, _payloadData, _configRec); err != nil {
			return nil, err
		}

		return _configRec, nil
	}

	return nil, nil
}

func (block *RawBlock) decodePayloadHeader(header *common.Header, _header *PayloadHeader) error {
	_channelHeader := &ChannelHeader{}
	_header.ChannelHeader = _channelHeader
	channelHeader := &common.ChannelHeader{}
	if err := proto.Unmarshal(header.ChannelHeader, channelHeader); err != nil {
		return errors.Wrap(err, "error decoding ChannelHeader from payload")
	}
	_channelHeader.ChannelId = channelHeader.ChannelId
	_channelHeader.Epoch = strconv.FormatUint(channelHeader.Epoch, 10)
	_channelHeader.Timestamp = time.Unix(channelHeader.Timestamp.GetSeconds(), int64(channelHeader.Timestamp.GetNanos())).UnixNano()
	_channelHeader.TxId = channelHeader.TxId
	_channelHeader.Type = common.HeaderType_name[channelHeader.Type]
	_channelHeader.Version = int(channelHeader.Version)

	_signatureHeader := &SignatureHeader{}
	_header.SignatureHeader = _signatureHeader
	err := block.decodeSignatureHeader(_signatureHeader, header.SignatureHeader)
	if err != nil {
		return errors.Wrap(err, "error decoding SignatureHeader from payload")
	}

	return nil
}

func (block *RawBlock) decodeSignatureHeader(_signatureHeader *SignatureHeader, bytes []byte) error {
	signatureHeader := &common.SignatureHeader{}
	if err := proto.Unmarshal(bytes, signatureHeader); err != nil {
		return errors.Wrap(err, "error decoding SignatureHeader from payload")
	}
	_signatureHeader.Nonce = base64.StdEncoding.EncodeToString(signatureHeader.Nonce)
	creator := &msp.SerializedIdentity{}
	if err := proto.Unmarshal(signatureHeader.Creator, creator); err != nil {
		return errors.Wrap(err, "error decoding Creator from signature header")
	}
	_signatureHeader.Creator = creator
	return nil
}

func (block *RawBlock) decodeTxPayloadData(payloadData []byte, _payloadData *PayloadData, _transaction *Transaction) error {
	tx := &peer.Transaction{}
	if err := proto.Unmarshal(payloadData, tx); err != nil {
		return errors.Wrap(err, "error decoding transaction payload data")
	}

	// each transaction may pack multiple proposals, for instance a DvP transaction that
	// includes both a proposal for the payment and a proposal for the delivery.
	_actions := make([]*Action, len(tx.Actions))
	_payloadData.Actions = _actions

	txActions := make([]*TransactionAction, len(tx.Actions))
	_transaction.Actions = txActions

	for i, action := range tx.Actions {
		_action := &Action{}
		_signatureHeader := &SignatureHeader{}
		_action.Header = _signatureHeader
		if err := block.decodeSignatureHeader(_signatureHeader, action.Header); err != nil {
			return err
		}
		_actions[i] = _action

		txAction := &TransactionAction{}
		txActions[i] = txAction

		_actionPayload := &ActionPayload{}
		_action.Payload = _actionPayload

		if err := block.decodeActionPayload(_actionPayload, action.Payload); err != nil {
			return err
		}

		txAction.Nonce = hex.EncodeToString([]byte(_signatureHeader.Nonce))
		txAction.ChaincodeId = _actionPayload.Action.ProposalResponsePayload.Extension.ChaincodeId
		creator := _signatureHeader.Creator
		txAction.Creator = &Creator{
			MspID: creator.Mspid,
			Cert:  string(creator.IdBytes),
		}
		txAction.Event = _actionPayload.Action.ProposalResponsePayload.Extension.Events
		if _actionPayload.ChaincodeProposalPayload.Input.ChaincodeSpec != nil {
			txAction.Input = _actionPayload.ChaincodeProposalPayload.Input.ChaincodeSpec.Input
		}
		txAction.ProposalHash = hex.EncodeToString([]byte(_actionPayload.Action.ProposalResponsePayload.ProposalHash))
		txAction.TransientMap = _actionPayload.ChaincodeProposalPayload.TransientMap
	}

	return nil
}

func (block *RawBlock) decodeActionPayload(_actionPayload *ActionPayload, payload []byte) error {
	_actionPayloadAction := &ActionPayloadAction{}
	_actionPayload.Action = _actionPayloadAction

	_chaincodeProposalPayload := &ChaincodeProposalPayload{}
	_actionPayload.ChaincodeProposalPayload = _chaincodeProposalPayload

	cap := &peer.ChaincodeActionPayload{}
	if err := proto.Unmarshal(payload, cap); err != nil {
		return errors.Wrap(err, "error decoding chaincode action payload")
	}

	if err := block.decodeActionPayloadAction(_actionPayloadAction, cap.Action); err != nil {
		return err
	}

	if err := block.decodeActionPayloadChaincodeProposalPayload(_chaincodeProposalPayload, cap.ChaincodeProposalPayload); err != nil {
		return err
	}

	return nil
}

func (block *RawBlock) decodeActionPayloadChaincodeProposalPayload(chaincodeProposalPayload *ChaincodeProposalPayload, bytes []byte) error {
	cpp := &peer.ChaincodeProposalPayload{}
	if err := proto.Unmarshal(bytes, cpp); err != nil {
		return errors.Wrap(err, "error decoding chaincode proposal payload")
	}

	chaincodeProposalPayload.TransientMap = &cpp.TransientMap

	proposalPayloadInput := &ProposalPayloadInput{}
	chaincodeProposalPayload.Input = proposalPayloadInput

	ccSpec := &peer.ChaincodeInvocationSpec{}
	if err := proto.Unmarshal(cpp.Input, ccSpec); err != nil {
		return errors.Wrap(err, "error decoding chaincode invocation spec")
	}

	if ccSpec.ChaincodeSpec != nil {
		chaincodeSpec := &ChaincodeSpec{}
		chaincodeSpec.Type = ccSpec.ChaincodeSpec.Type.String()
		proposalPayloadInput.ChaincodeSpec = chaincodeSpec

		chaincodeSpec.ChaincodeId = ccSpec.ChaincodeSpec.ChaincodeId

		chaincodeInput := &ChaincodeSpecInput{}
		chaincodeSpec.Input = chaincodeInput

		chaincodeInputArgs := make([]string, len(ccSpec.ChaincodeSpec.Input.Args))
		for j, arg := range ccSpec.ChaincodeSpec.Input.Args {
			chaincodeInputArgs[j] = string(arg)
		}
		chaincodeInput.Args = chaincodeInputArgs
		chaincodeInput.IsInit = ccSpec.ChaincodeSpec.Input.IsInit
	}

	return nil
}

func (block *RawBlock) decodeActionPayloadAction(_actionPayloadAction *ActionPayloadAction, action *peer.ChaincodeEndorsedAction) error {
	_proposalResponsePayload := &ProposalResponsePayload{}
	_actionPayloadAction.ProposalResponsePayload = _proposalResponsePayload
	if err := block.decodeProposalResponsePayload(_proposalResponsePayload, action.ProposalResponsePayload); err != nil {
		return err
	}
	return nil
}

func (block *RawBlock) decodeProposalResponsePayload(_proposalResponsePayload *ProposalResponsePayload, proposalResponsePayload []byte) error {
	prp := &peer.ProposalResponsePayload{}
	if err := proto.Unmarshal(proposalResponsePayload, prp); err != nil {
		return errors.Wrap(err, "error decoding chaincode proposal response payload")
	}

	_proposalResponsePayload.ProposalHash = base64.StdEncoding.EncodeToString(prp.ProposalHash)

	_extension := &Extension{}
	_proposalResponsePayload.Extension = _extension
	if err := block.decodeProposalResponsePayloadExtension(_extension, prp.Extension); err != nil {
		return err
	}
	return nil
}

func (block *RawBlock) decodeProposalResponsePayloadExtension(_extension *Extension, extension []byte) error {
	cca := &peer.ChaincodeAction{}
	if err := proto.Unmarshal(extension, cca); err != nil {
		return errors.Wrap(err, "error decoding chaincode action")
	}

	_extension.ChaincodeId = cca.ChaincodeId

	// skipping read/write sets in the action
	// decode events
	ccevt := &peer.ChaincodeEvent{}
	if err := proto.Unmarshal(cca.Events, ccevt); err != nil {
		return errors.Wrap(err, "error decoding chaincode event")
	}
	_chaincodeEvent := &ChaincodeEvent{}
	_extension.Events = _chaincodeEvent
	_chaincodeEvent.ChaincodeId = ccevt.ChaincodeId
	_chaincodeEvent.TxId = ccevt.TxId
	_chaincodeEvent.EventName = ccevt.EventName
	_chaincodeEvent.Payload = utils.DecodePayload(ccevt.Payload)

	return nil
}

func (block *RawBlock) decodeConfigPayloadData(payloadData []byte, _payloadData *PayloadData, _configRec *ConfigRecord) error {
	configEnv := &common.ConfigEnvelope{}
	if err := proto.Unmarshal(payloadData, configEnv); err != nil {
		return errors.Wrap(err, "error decoding config envelope")
	}
	_configRec.Config = configEnv.Config
	_payloadData.Config = configEnv.Config

	return nil
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
