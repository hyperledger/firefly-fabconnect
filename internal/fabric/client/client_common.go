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

package client

import (
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/utils"
	log "github.com/sirupsen/logrus"
)

type commonRPCWrapper struct {
	txTimeout           int
	configProvider      core.ConfigProvider
	sdk                 *fabsdk.FabricSDK
	idClient            IdentityClient
	ledgerClientWrapper *ledgerClientWrapper
	eventClientWrapper  *eventClientWrapper
	channelCreator      channelCreator
}

func getOrgFromConfig(config core.ConfigProvider) (string, error) {
	configBackend, err := config()
	if err != nil {
		return "", err
	}
	if len(configBackend) != 1 {
		return "", errors.Errorf("Invalid config file")
	}

	cfg := configBackend[0]
	value, ok := cfg.Lookup("client.organization")
	if !ok {
		return "", errors.Errorf("No client organization defined in the config")
	}

	return value.(string), nil
}

func getFirstPeerEndpointFromConfig(config core.ConfigProvider) (string, error) {
	org, err := getOrgFromConfig(config)
	if err != nil {
		return "", err
	}
	configBackend, _ := config()
	cfg := configBackend[0]
	value, ok := cfg.Lookup(fmt.Sprintf("organizations.%s.peers", org))
	if !ok {
		return "", errors.Errorf("No peers list found in the organization %s", org)
	}
	peers := value.([]interface{})
	if len(peers) < 1 {
		return "", errors.Errorf("Peers list for organization %s is empty", org)
	}
	return peers[0].(string), nil
}

// defined to allow mocking in tests
type channelCreator func(context.ChannelProvider) (*channel.Client, error)

func createChannelClient(channelProvider context.ChannelProvider) (*channel.Client, error) {
	return channel.New(channelProvider)
}

func newReceipt(responsePayload []byte, status *fab.TxStatusEvent, signerID *msp.IdentityIdentifier) *TxReceipt {
	return &TxReceipt{
		SignerMSP:     signerID.MSPID,
		Signer:        signerID.ID,
		TransactionID: status.TxID,
		Status:        status.TxValidationCode,
		BlockNumber:   status.BlockNumber,
		SourcePeer:    status.SourceURL,
	}
}

func convertStringArray(args []string) [][]byte {
	result := [][]byte{}
	for _, v := range args {
		result = append(result, []byte(v))
	}
	return result
}

func convertStringMap(_map map[string]string) map[string][]byte {
	result := make(map[string][]byte, len(_map))
	for k, v := range _map {
		result[k] = []byte(v)
	}
	return result
}

func (w *commonRPCWrapper) QueryChainInfo(channelID, signer string) (*fab.BlockchainInfoResponse, error) {
	log.Tracef("RPC [%s] --> QueryChainInfo", channelID)

	result, err := w.ledgerClientWrapper.queryChainInfo(channelID, signer)
	if err != nil {
		log.Errorf("Failed to query chain info on channel %s. %s", channelID, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelID, result)
	return result, nil
}

func (w *commonRPCWrapper) QueryBlock(channelID string, signer string, blockNumber uint64, blockhash []byte) (*utils.RawBlock, *utils.Block, error) {
	log.Tracef("RPC [%s] --> QueryBlock %v", channelID, blockNumber)

	rawblock, block, err := w.ledgerClientWrapper.queryBlock(channelID, signer, blockNumber, blockhash)
	if err != nil {
		log.Errorf("Failed to query block %v on channel %s. %s", blockNumber, channelID, err)
		return nil, nil, err
	}

	log.Tracef("RPC [%s] <-- success", channelID)
	return rawblock, block, nil
}

func (w *commonRPCWrapper) QueryBlockByTxID(channelID string, signer string, txID string) (*utils.RawBlock, *utils.Block, error) {
	log.Tracef("RPC [%s] --> QueryBlockByTxID %s", channelID, txID)

	rawblock, block, err := w.ledgerClientWrapper.queryBlockByTxID(channelID, signer, txID)
	if err != nil {
		log.Errorf("Failed to query block by transaction Id %s on channel %s. %s", txID, channelID, err)
		return nil, nil, err
	}

	log.Tracef("RPC [%s] <-- success", channelID)
	return rawblock, block, nil
}

func (w *commonRPCWrapper) QueryTransaction(channelID, signer, txID string) (map[string]interface{}, error) {
	log.Tracef("RPC [%s] --> QueryTransaction %s", channelID, txID)

	result, err := w.ledgerClientWrapper.queryTransaction(channelID, signer, txID)
	if err != nil {
		log.Errorf("Failed to query transaction on channel %s. %s", channelID, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelID, result)
	return result, nil
}

// The returned registration must be closed when done
func (w *commonRPCWrapper) SubscribeEvent(subInfo *eventsapi.SubscriptionInfo, since uint64) (*RegistrationWrapper, <-chan *fab.BlockEvent, <-chan *fab.CCEvent, error) {
	reg, blockEventCh, ccEventCh, err := w.eventClientWrapper.subscribeEvent(subInfo, since)
	if err != nil {
		log.Errorf("Failed to subscribe to event [%s:%s:%s]. %s", subInfo.Stream, subInfo.ChannelID, subInfo.Filter.ChaincodeID, err)
		return nil, nil, nil, err
	}
	return reg, blockEventCh, ccEventCh, nil
}

func (w *commonRPCWrapper) Unregister(regWrapper *RegistrationWrapper) {
	regWrapper.eventClient.Unregister(regWrapper.registration)
}
