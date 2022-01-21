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
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/peer"
)

type RawBlock struct {
	Data     *BlockData            `json:"data"`
	Header   *common.BlockHeader   `json:"header"`
	Metadata *common.BlockMetadata `json:"metadata"`
}

type BlockData struct {
	Data []*BlockDataEnvelope `json:"data"`
}

type BlockDataEnvelope struct {
	Payload   *Payload `json:"payload"`
	Signature string   `json:"signature"`
}

type Payload struct {
	Data   *PayloadData   `json:"data"`
	Header *PayloadHeader `json:"header"`
}

type PayloadHeader struct {
	ChannelHeader   *ChannelHeader   `json:"channel_header"`
	SignatureHeader *SignatureHeader `json:"signature_header"`
}

type ChannelHeader struct {
	ChannelId string `json:"channel_id"`
	Epoch     string `json:"epoch"`
	Timestamp int64  `json:"timestamp"`
	TxId      string `json:"tx_id"`
	Type      string `json:"type"`
	Version   int    `json:"version"`
}

type SignatureHeader struct {
	Creator *msp.SerializedIdentity `json:"creator"`
	Nonce   string                  `json:"nonce"`
}

type PayloadData struct {
	// Actions only exists in endorsement transaction blocks
	Actions []*Action `json:"actions,omitempty"`
	// Config only exists in config and config update blocks
	Config *common.Config `json:"config,omitempty"`
}

//
// Types used only in endorsement transaction blocks
//

type Action struct {
	Header  *SignatureHeader `json:"header"`
	Payload *ActionPayload   `json:"payload"`
}

type ActionPayload struct {
	Action                   *ActionPayloadAction      `json:"action"`
	ChaincodeProposalPayload *ChaincodeProposalPayload `json:"chaincode_proposal_payload"`
}

type ActionPayloadAction struct {
	// Endorsements are not parsed for now
	ProposalResponsePayload *ProposalResponsePayload `json:"proposal_response_payload"`
}

type ProposalResponsePayload struct {
	Extension    *Extension `json:"extension"`
	ProposalHash string     `json:"proposal_hash"`
}

type Extension struct {
	ChaincodeId *peer.ChaincodeID `json:"chaincode_id"`
	Events      *ChaincodeEvent   `json:"events"`
	// Response
	// Result
}

type ChaincodeEvent struct {
	ChaincodeId string      `json:"chaincodeId"`
	TxId        string      `json:"transactionId"`
	Timestamp   string      `json:"timestamp"`
	EventName   string      `json:"eventName"`
	Payload     interface{} `json:"payload"`
}

type ChaincodeProposalPayload struct {
	TransientMap *map[string][]byte    `json:"TransientMap"`
	Input        *ProposalPayloadInput `json:"input"`
}

type ProposalPayloadInput struct {
	ChaincodeSpec *ChaincodeSpec `json:"chaincode_spec"`
}

type ChaincodeSpec struct {
	ChaincodeId *peer.ChaincodeID   `json:"chaincode_id"`
	Input       *ChaincodeSpecInput `json:"input"`
	Type        string              `json:"type"`
}

type ChaincodeSpecInput struct {
	Args   []string `json:"args"`
	IsInit bool     `json:"is_init"`
}
