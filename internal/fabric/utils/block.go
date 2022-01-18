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
	"github.com/hyperledger/fabric-protos-go/peer"
)

type Block struct {
	Number       uint64 `json:"block_number"`
	DataHash     string `json:"data_hash"`
	PreviousHash string `json:"previous_hash"`

	// only for endorser transaction blocks
	Transactions []*Transaction `json:"transactions,omitempty"`
	// only for Config blocks
	Config *ConfigRecord `json:"config,omitempty"`
}

type Creator struct {
	MspID string `json:"msp_id"`
	Cert  string `json:"cert"`
}

type Transaction struct {
	Type      string               `json:"type"`
	TxId      string               `json:"tx_id"`
	Nonce     string               `json:"nonce"` // hex string
	Creator   *Creator             `json:"creator"`
	Status    string               `json:"status"`
	Signature string               `json:"signature"`
	Timestamp int64                `json:"timestamp"` // unix nano
	Actions   []*TransactionAction `json:"actions"`
}

type TransactionAction struct {
	Nonce        string              `json:"nonce"` // hex string
	Creator      *Creator            `json:"creator"`
	TransientMap *map[string][]byte  `json:"transient_map"`
	ChaincodeId  *peer.ChaincodeID   `json:"chaincode_id"`
	Input        *ChaincodeSpecInput `json:"input"`
	ProposalHash string              `json:"proposal_hash"` // hex string
	Event        *ChaincodeEvent     `json:"event"`
}

type ConfigRecord struct {
	Type      string         `json:"type"`
	Signature string         `json:"signature"`
	Timestamp int64          `json:"timestamp"` // unix nano
	Nonce     string         `json:"nonce"`     // hex string
	Creator   *Creator       `json:"creator"`
	Config    *common.Config `json:"config"`
}
