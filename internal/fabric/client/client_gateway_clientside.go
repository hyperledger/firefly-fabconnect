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
	"context"
	"time"

	pb "github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	log "github.com/sirupsen/logrus"
)

// defined to allow mocking in tests
type gatewayCreator func(core.ConfigProvider, string, int) (*gateway.Gateway, error)
type networkCreator func(*gateway.Gateway, string) (*gateway.Network, error)

type gwRPCWrapper struct {
	txTimeout           int
	configProvider      core.ConfigProvider
	idClient            IdentityClient
	ledgerClientWrapper *ledgerClientWrapper
	eventClientWrapper  *eventClientWrapper
	gatewayCreator      gatewayCreator
	networkCreator      networkCreator
	// networkCreator networkC
	// one gateway client per signer
	gwClients map[string]*gateway.Gateway
	// one gateway network per signer per channel
	gwChannelClients map[string]map[string]*gateway.Network
}

func newRPCClientWithClientSideGateway(configProvider core.ConfigProvider, txTimeout int, idClient IdentityClient, ledgerClientWrapper *ledgerClientWrapper, eventClientWrapper *eventClientWrapper) (RPCClient, error) {
	return &gwRPCWrapper{
		txTimeout:           txTimeout,
		configProvider:      configProvider,
		idClient:            idClient,
		ledgerClientWrapper: ledgerClientWrapper,
		eventClientWrapper:  eventClientWrapper,
		gatewayCreator:      createGateway,
		networkCreator:      getNetwork,
		gwClients:           make(map[string]*gateway.Gateway),
		gwChannelClients:    make(map[string]map[string]*gateway.Network),
	}, nil
}

func (w *gwRPCWrapper) Invoke(channelId, signer, chaincodeName, method string, args []string, isInit bool) (*TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s:isInit=%t] --> %+v", channelId, chaincodeName, method, isInit, args)

	result, txStatus, err := w.sendTransaction(channelId, signer, chaincodeName, method, args, false)
	if err != nil {
		log.Errorf("Failed to send transaction [%s:%s:%s:isInit=%t]. %s", channelId, chaincodeName, method, isInit, err)
		return nil, err
	}
	signingId, err := w.idClient.GetSigningIdentity(signer)
	if err != nil {
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s:isInit=%t] <-- %+v", channelId, chaincodeName, method, isInit, result)
	return newReceipt(result, txStatus, signingId.Identifier()), err
}

func (w *gwRPCWrapper) Query(channelId, signer, chaincodeName, method string, args []string) ([]byte, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}

	contractClient := client.GetContract(chaincodeName)
	result, err := contractClient.EvaluateTransaction(method, args...)
	if err != nil {
		log.Errorf("Failed to send query [%s:%s:%s]. %s", channelId, chaincodeName, method, err)
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return result, nil
}

func (w *gwRPCWrapper) QueryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error) {
	log.Tracef("RPC [%s] --> ChainInfo", channelId)

	result, err := w.ledgerClientWrapper.queryChainInfo(channelId, signer)
	if err != nil {
		log.Errorf("Failed to query chain info on channel %s. %s", channelId, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

func (w *gwRPCWrapper) QueryTransaction(channelId, signer, txId string) (*pb.FilteredTransaction, error) {
	log.Tracef("RPC [%s] --> QueryTransaction %s", channelId, txId)

	result, err := w.ledgerClientWrapper.queryTransaction(channelId, signer, txId)
	if err != nil {
		log.Errorf("Failed to query transaction on channel %s. %s", channelId, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

// The returned registration must be closed when done
func (w *gwRPCWrapper) SubscribeEvent(subInfo *eventsapi.SubscriptionInfo, since uint64) (*RegistrationWrapper, <-chan *fab.BlockEvent, <-chan *fab.CCEvent, error) {
	reg, blockEventCh, ccEventCh, err := w.eventClientWrapper.subscribeEvent(subInfo, since)
	if err != nil {
		log.Errorf("Failed to subscribe to event [%s:%s:%s]. %s", subInfo.Stream, subInfo.ChannelId, subInfo.Filter.ChaincodeId, err)
		return nil, nil, nil, err
	}
	return reg, blockEventCh, ccEventCh, nil
}

func (w *gwRPCWrapper) Unregister(regWrapper *RegistrationWrapper) {
	regWrapper.eventClient.Unregister(regWrapper.registration)
}

func (w *gwRPCWrapper) Close() error {
	// the ledgerClientWrapper and the eventClientWrapper share the same sdk instance
	// only need to close it from one of them
	w.ledgerClientWrapper.sdk.Close()
	return nil
}

func (w *gwRPCWrapper) sendTransaction(signer, channelId, chaincodeName, method string, args []string, isInit bool) ([]byte, *fab.TxStatusEvent, error) {
	channelClient, err := w.getChannelClient(signer, channelId)
	if err != nil {
		return nil, nil, err
	}
	contractClient := channelClient.GetContract(chaincodeName)
	tx, err := contractClient.CreateTransaction(method)
	if err != nil {
		return nil, nil, err
	}
	notifier := tx.RegisterCommitEvent()
	result, err := tx.Submit(args...)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(w.txTimeout)*time.Second)
	select {
	case txStatus := <-notifier:
		cancel()
		return result, txStatus, nil
	case <-ctx.Done():
		cancel()
		return nil, nil, errors.Errorf("Failed to get status event for transaction (channel=%s, chaincode=%s, func=%s)", channelId, chaincodeName, method)
	}
}

func (w *gwRPCWrapper) getChannelClient(channelId, signer string) (channelClient *gateway.Network, err error) {
	channelClientsForSigner := w.gwChannelClients[signer]
	if channelClientsForSigner == nil {
		// no channel clients have been created for this signer at all
		// we will not have created a gateway client for this user either
		gatewayClient, err := w.gatewayCreator(w.configProvider, signer, w.txTimeout)
		if err != nil {
			return nil, err
		}
		w.gwClients[signer] = gatewayClient
		channelClientsForSigner = make(map[string]*gateway.Network)
		w.gwChannelClients[signer] = channelClientsForSigner
	}

	channelClient = channelClientsForSigner[channelId]
	if channelClient == nil {
		client := w.gwClients[signer]
		channelClient, err = w.networkCreator(client, channelId)
		if err != nil {
			return nil, err
		}
		channelClientsForSigner[channelId] = channelClient
	}
	return channelClient, nil
}

func createGateway(configProvider core.ConfigProvider, signer string, txTimeout int) (*gateway.Gateway, error) {
	return gateway.Connect(gateway.WithConfig(configProvider), gateway.WithUser(signer), gateway.WithTimeout(time.Duration(txTimeout)*time.Second))
}

func getNetwork(gateway *gateway.Gateway, channelId string) (*gateway.Network, error) {
	return gateway.GetNetwork(channelId)
}
