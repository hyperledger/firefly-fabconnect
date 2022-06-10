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
	"sync"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	log "github.com/sirupsen/logrus"
)

// defined to allow mocking in tests
type gatewayCreator func(core.ConfigProvider, string, int) (*gateway.Gateway, error)
type networkCreator func(*gateway.Gateway, string) (*gateway.Network, error)
type txPreparer func(*gwRPCWrapper, string, string, string, string) (*gateway.Transaction, <-chan *fab.TxStatusEvent, error)
type txSubmitter func(*gateway.Transaction, ...string) ([]byte, error)

type gwRPCWrapper struct {
	*commonRPCWrapper
	gatewayCreator  gatewayCreator
	networkCreator  networkCreator
	txPreparer      txPreparer
	txSubmitter     txSubmitter
	txInitSubmitter txSubmitter
	// networkCreator networkC
	// one gateway client per signer
	gwClients map[string]*gateway.Gateway
	// one gateway network per signer per channel
	gwGatewayClients map[string]map[string]*gateway.Network
	// one channel client per signer per channel
	gwChannelClients map[string]map[string]*channel.Client
	mu               sync.Mutex
}

func newRPCClientWithClientSideGateway(configProvider core.ConfigProvider, txTimeout int, idClient IdentityClient, ledgerClientWrapper *ledgerClientWrapper, eventClientWrapper *eventClientWrapper) (RPCClient, error) {
	w := &gwRPCWrapper{
		commonRPCWrapper: &commonRPCWrapper{
			txTimeout:           txTimeout,
			configProvider:      configProvider,
			idClient:            idClient,
			ledgerClientWrapper: ledgerClientWrapper,
			eventClientWrapper:  eventClientWrapper,
			channelCreator:      createChannelClient,
		},
		gatewayCreator:   createGateway,
		networkCreator:   getNetwork,
		txPreparer:       prepareTx,
		txSubmitter:      submitTx,
		txInitSubmitter:  submitInitTx,
		gwClients:        make(map[string]*gateway.Gateway),
		gwGatewayClients: make(map[string]map[string]*gateway.Network),
		gwChannelClients: make(map[string]map[string]*channel.Client),
	}

	idClient.AddSignerUpdateListener(w)
	return w, nil
}

func (w *gwRPCWrapper) Invoke(channelId, signer, chaincodeName, method string, args []string, isInit bool) (*TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s:isInit=%t] --> %+v", channelId, chaincodeName, method, isInit, args)

	result, txStatus, err := w.sendTransaction(channelId, signer, chaincodeName, method, args, isInit)
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

func (w *gwRPCWrapper) Query(channelId, signer, chaincodeName, method string, args []string, strongread bool) ([]byte, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}

	if strongread {
		client, err := w.getGatewayClient(channelId, signer)
		if err != nil {
			return nil, errors.Errorf("Failed to get gateway client. %s", err)
		}
		contractClient := client.GetContract(chaincodeName)
		result, err := contractClient.EvaluateTransaction(method, args...)
		if err != nil {
			log.Errorf("Failed to send query [%s:%s:%s]. %s", channelId, chaincodeName, method, err)
			return nil, err
		}

		log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
		return result, nil
	} else {
		peerEndpoint, err := getFirstPeerEndpointFromConfig(w.configProvider)
		if err != nil {
			return nil, err
		}

		bytes := convert(args)
		req := channel.Request{
			ChaincodeID: chaincodeName,
			Fcn:         method,
			Args:        bytes,
		}
		result, err := client.Query(req, channel.WithRetry(retry.DefaultChannelOpts), channel.WithTargetEndpoints(peerEndpoint))
		if err != nil {
			log.Errorf("Failed to send query [%s:%s:%s]. %s", channelId, chaincodeName, method, err)
			return nil, err
		}

		log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
		return result.Payload, nil
	}
}

func (w *gwRPCWrapper) SignerUpdated(signer string) {
	w.mu.Lock()
	w.gwClients[signer] = nil
	w.gwGatewayClients[signer] = nil
	w.gwChannelClients[signer] = nil
	w.mu.Unlock()
}

func (w *gwRPCWrapper) Close() error {
	// the ledgerClientWrapper and the eventClientWrapper share the same sdk instance
	// only need to close it from one of them
	w.ledgerClientWrapper.sdk.Close()
	return nil
}

func (w *gwRPCWrapper) sendTransaction(signer, channelId, chaincodeName, method string, args []string, isInit bool) ([]byte, *fab.TxStatusEvent, error) {
	tx, notifier, err := w.txPreparer(w, signer, channelId, chaincodeName, method)
	if err != nil {
		return nil, nil, err
	}
	var result []byte
	if isInit {
		result, err = w.txInitSubmitter(tx, args...)
	} else {
		result, err = w.txSubmitter(tx, args...)
	}
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

// channel clients for transactions are created with the gateway API, so that the internal handling of using
// the discovery service and selecting the right set of endorsers are automated
func (w *gwRPCWrapper) getGatewayClient(channelId, signer string) (gatewayClient *gateway.Network, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	gatewayClientsForSigner := w.gwGatewayClients[signer]
	if gatewayClientsForSigner == nil {
		// no channel clients have been created for this signer at all
		// we will not have created a gateway client for this user either
		gatewayClient, err := w.gatewayCreator(w.configProvider, signer, w.txTimeout)
		if err != nil {
			return nil, err
		}
		w.gwClients[signer] = gatewayClient
		gatewayClientsForSigner = make(map[string]*gateway.Network)
		w.gwGatewayClients[signer] = gatewayClientsForSigner
	}

	gatewayClient = gatewayClientsForSigner[channelId]
	if gatewayClient == nil {
		client := w.gwClients[signer]
		gatewayClient, err = w.networkCreator(client, channelId)
		if err != nil {
			return nil, err
		}
		gatewayClientsForSigner[channelId] = gatewayClient
	}
	return gatewayClient, nil
}

// channel clients for queries are created with the channel client API, so that we can dictate the target
// peer to be the single peer that this fabconnect instance is attached to. This is more useful than trying to
// do a "strong read" across multiple peers
func (w *gwRPCWrapper) getChannelClient(channelId, signer string) (channelClient *channel.Client, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	channelClientsForSigner := w.gwChannelClients[signer]
	if channelClientsForSigner == nil {
		channelClientsForSigner = make(map[string]*channel.Client)
		w.gwChannelClients[signer] = channelClientsForSigner
	}

	channelClient = channelClientsForSigner[channelId]
	if channelClient == nil {
		sdk := w.ledgerClientWrapper.sdk
		org, err := getOrgFromConfig(w.configProvider)
		if err != nil {
			return nil, err
		}
		clientChannelContext := sdk.ChannelContext(channelId, fabsdk.WithUser(signer), fabsdk.WithOrg(org))
		// Channel client is used to query and execute transactions (Org1 is default org)
		channelClient, err = w.channelCreator(clientChannelContext)
		if err != nil {
			return nil, errors.Errorf("Failed to create new channel client: %s", err)
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

func prepareTx(w *gwRPCWrapper, signer, channelId, chaincodeName, method string) (*gateway.Transaction, <-chan *fab.TxStatusEvent, error) {
	channelClient, err := w.getGatewayClient(signer, channelId)
	if err != nil {
		return nil, nil, err
	}
	contractClient := channelClient.GetContract(chaincodeName)
	tx, err := contractClient.CreateTransaction(method)
	if err != nil {
		return nil, nil, err
	}
	notifier := tx.RegisterCommitEvent()
	return tx, notifier, nil
}

func submitTx(tx *gateway.Transaction, args ...string) ([]byte, error) {
	return tx.Submit(args...)
}

func submitInitTx(tx *gateway.Transaction, args ...string) ([]byte, error) {
	return tx.SubmitInit(args...)
}
