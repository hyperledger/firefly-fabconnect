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
	eventsapi "github.com/hyperledger-labs/firefly-fabconnect/internal/events/api"
	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/event"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
)

type ChaincodeSpec struct {
	Type    int    `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type TxReceipt struct {
	BlockNumber     uint64              `json:"blockNumber"`
	SignerMSP       string              `json:"signerMSP"`
	Signer          string              `json:"signer"`
	ChaincodeSpec   ChaincodeSpec       `json:"chaincode"`
	TransactionID   string              `json:"transactionID"`
	Status          pb.TxValidationCode `json:"status"`
	SourcePeer      string              `json:"peer"`
	ResponsePayload []byte              `json:"responsePayload"`
}

func (r *TxReceipt) IsSuccess() bool {
	return r.Status == pb.TxValidationCode_VALID
}

type RegistrationWrapper struct {
	registration fab.Registration
	eventClient  *event.Client
}

type RPCClient interface {
	Invoke(channelId, signer, chaincodeName, method string, args []string, isInit bool) (*TxReceipt, error)
	Query(channelId, signer, chaincodeName, method string, args []string) ([]byte, error)
	QueryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error)
	SubscribeEvent(subInfo *eventsapi.SubscriptionInfo, since uint64) (*RegistrationWrapper, <-chan *fab.BlockEvent, <-chan *fab.CCEvent, error)
	Unregister(*RegistrationWrapper)
	Close() error
}

type IdentityClient interface {
	GetSigningIdentity(name string) (msp.SigningIdentity, error)
	GetClientOrg() string
}
