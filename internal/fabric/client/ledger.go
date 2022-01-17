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
	//nolint

	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/utils"
)

// defined to allow mocking in tests
type ledgerClientCreator func(channelProvider context.ChannelProvider, opts ...ledger.ClientOption) (*ledger.Client, error)

type ledgerClientWrapper struct {
	// ledger client per channel per signer
	ledgerClients       map[string]map[string]*ledger.Client
	sdk                 *fabsdk.FabricSDK
	idClient            IdentityClient
	ledgerClientCreator ledgerClientCreator
}

func newLedgerClient(configProvider core.ConfigProvider, sdk *fabsdk.FabricSDK, idClient IdentityClient) *ledgerClientWrapper {
	return &ledgerClientWrapper{
		sdk:                 sdk,
		idClient:            idClient,
		ledgerClients:       make(map[string]map[string]*ledger.Client),
		ledgerClientCreator: createLedgerClient,
	}
}

func (l *ledgerClientWrapper) queryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error) {
	client, err := l.getLedgerClient(channelId, signer)
	if err != nil {
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}
	result, err := client.QueryInfo()
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (l *ledgerClientWrapper) queryBlock(channelId string, blockNumber uint64, signer string) (*utils.RawBlock, *utils.Block, error) {
	client, err := l.getLedgerClient(channelId, signer)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to get channel client. %s", err)
	}
	result, err := client.QueryBlock(blockNumber)
	if err != nil {
		return nil, nil, err
	}
	rawblock, block, err := utils.DecodeBlock(result)
	return rawblock, block, err
}

func (l *ledgerClientWrapper) queryTransaction(channelId, signer, txId string) (map[string]interface{}, error) {
	client, err := l.getLedgerClient(channelId, signer)
	if err != nil {
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}
	txID := fab.TransactionID(txId)
	result, err := client.QueryTransaction(txID)
	if err != nil {
		return nil, err
	}
	bloc := &utils.RawBlock{}
	envelope, tx, err := bloc.DecodeBlockDataEnvelope(result.TransactionEnvelope)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]interface{})
	ret["transaction"] = tx
	ret["raw"] = envelope
	return ret, nil
}

func (l *ledgerClientWrapper) getLedgerClient(channelId, signer string) (ledgerClient *ledger.Client, err error) {
	ledgerClientsForSigner := l.ledgerClients[signer]
	if ledgerClientsForSigner == nil {
		ledgerClientsForSigner = make(map[string]*ledger.Client)
		l.ledgerClients[signer] = ledgerClientsForSigner
	}
	ledgerClient = ledgerClientsForSigner[channelId]
	if ledgerClient == nil {
		channelProvider := l.sdk.ChannelContext(channelId, fabsdk.WithOrg(l.idClient.GetClientOrg()), fabsdk.WithUser(signer))
		ledgerClient, err = l.ledgerClientCreator(channelProvider)
		if err != nil {
			return nil, err
		}
		ledgerClientsForSigner[channelId] = ledgerClient
	}
	return ledgerClient, nil
}

func createLedgerClient(channelProvider context.ChannelProvider, opts ...ledger.ClientOption) (*ledger.Client, error) {
	return ledger.New(channelProvider, opts...)
}
