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
	"sync"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel/invoke"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	mspImpl "github.com/hyperledger/fabric-sdk-go/pkg/msp"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	log "github.com/sirupsen/logrus"
)

// the ccpClientWrapper implements the RPCClient interface using the fabric-sdk-go implementation
// based on the static network description provided via the CCP yaml
type ccpClientWrapper struct {
	channelClient   *channel.Client
	channelProvider context.ChannelProvider
	signer          *msp.IdentityIdentifier
}

type ccpRPCWrapper struct {
	*commonRPCWrapper
	cryptoSuiteConfig core.CryptoSuiteConfig
	userStore         msp.UserStore
	// one channel client per channel ID, per signer ID
	channelClients map[string](map[string]*ccpClientWrapper)
	mu             sync.Mutex
}

func newRPCClientFromCCP(configProvider core.ConfigProvider, txTimeout int, userStore msp.UserStore, idClient IdentityClient, ledgerClientWrapper *ledgerClientWrapper, eventClientWrapper *eventClientWrapper) (RPCClient, error) {
	configBackend, _ := configProvider()
	cryptoConfig := cryptosuite.ConfigFromBackend(configBackend...)
	identityConfig, err := mspImpl.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, errors.Errorf("Failed to load identity configurations: %s", err)
	}
	clientConfig := identityConfig.Client()
	if clientConfig.CredentialStore.Path == "" {
		return nil, errors.Errorf("User credentials store path is empty")
	}

	log.Infof("New gRPC connection established")
	w := &ccpRPCWrapper{
		commonRPCWrapper: &commonRPCWrapper{
			sdk:                 ledgerClientWrapper.sdk,
			configProvider:      configProvider,
			idClient:            idClient,
			ledgerClientWrapper: ledgerClientWrapper,
			eventClientWrapper:  eventClientWrapper,
			channelCreator:      createChannelClient,
			txTimeout:           txTimeout,
		},
		cryptoSuiteConfig: cryptoConfig,
		userStore:         userStore,
		channelClients:    make(map[string]map[string]*ccpClientWrapper),
	}

	idClient.AddSignerUpdateListener(w)
	return w, nil
}

func (w *ccpRPCWrapper) Invoke(channelId, signer, chaincodeName, method string, args []string, transientMap map[string]string, isInit bool) (*TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s:isInit=%t] --> %+v", channelId, chaincodeName, method, isInit, args)

	signerID, result, txStatus, err := w.sendTransaction(channelId, signer, chaincodeName, method, args, transientMap, isInit)
	if err != nil {
		log.Errorf("Failed to send transaction [%s:%s:%s:isInit=%t]. %s", channelId, chaincodeName, method, isInit, err)
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s:isInit=%t] <-- %+v", channelId, chaincodeName, method, isInit, result)
	return newReceipt(result, txStatus, signerID), err
}

func (w *ccpRPCWrapper) Query(channelId, signer, chaincodeName, method string, args []string, strongread bool) ([]byte, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		log.Errorf("Failed to get channel client. %s", err)
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}

	req := channel.Request{
		ChaincodeID: chaincodeName,
		Fcn:         method,
		Args:        convertStringArray(args),
	}

	var result channel.Response
	var err1 error
	if strongread {
		// strongread means querying a set of peers that would have fulfilled the
		// endorsement policies and make sure they all have the same results
		result, err1 = client.channelClient.Query(req, channel.WithRetry(retry.DefaultChannelOpts))
	} else {
		peerEndpoint, err := getFirstPeerEndpointFromConfig(w.configProvider)
		if err != nil {
			return nil, err
		}
		result, err1 = client.channelClient.Query(req, channel.WithRetry(retry.DefaultChannelOpts), channel.WithTargetEndpoints(peerEndpoint))
	}
	if err1 != nil {
		log.Errorf("Failed to send query [%s:%s:%s]. %s", channelId, chaincodeName, method, err)
		return nil, err1
	}

	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return result.Payload, nil
}

func (w *ccpRPCWrapper) SignerUpdated(signer string) {
	w.mu.Lock()
	for _, clientsOfChannel := range w.channelClients {
		clientsOfChannel[signer] = nil
	}
	w.mu.Unlock()
}

func (w *ccpRPCWrapper) getChannelClient(channelId string, signer string) (*ccpClientWrapper, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	id, err := w.idClient.GetSigningIdentity(signer)
	if err == msp.ErrUserNotFound {
		return nil, errors.Errorf("Signer %s does not exist", signer)
	}
	if err != nil {
		return nil, errors.Errorf("Failed to retrieve signing identity: %s", err)
	}

	allClientsOfChannel := w.channelClients[channelId]
	if allClientsOfChannel == nil {
		w.channelClients[channelId] = make(map[string]*ccpClientWrapper)
	}
	clientOfUser := w.channelClients[channelId][id.Identifier().ID]
	if clientOfUser == nil {
		channelProvider := w.sdk.ChannelContext(channelId, fabsdk.WithOrg(w.idClient.GetClientOrg()), fabsdk.WithUser(id.Identifier().ID))
		cClient, err := w.channelCreator(channelProvider)
		if err != nil {
			return nil, err
		}
		newWrapper := &ccpClientWrapper{
			channelClient:   cClient,
			channelProvider: channelProvider,
			signer:          id.Identifier(),
		}
		w.channelClients[channelId][id.Identifier().ID] = newWrapper
		clientOfUser = newWrapper
	}
	return clientOfUser, nil
}

func (w *ccpRPCWrapper) Close() error {
	w.sdk.Close()
	return nil
}

func (w *ccpRPCWrapper) sendTransaction(channelId, signer, chaincodeName, method string, args []string, transientMap map[string]string, isInit bool) (*msp.IdentityIdentifier, []byte, *fab.TxStatusEvent, error) {
	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		return nil, nil, nil, errors.Errorf("Failed to get channel client. %s", err)
	}
	// in order to hook into the event notification for the transaction, we need to register
	// by the transaction ID before the transaction is sent to the orderer. Thus we can't use
	// the Execute() method of the client that consumes the event notification
	txStatus := fab.TxStatusEvent{}
	handlerChain := invoke.NewSelectAndEndorseHandler(
		invoke.NewEndorsementValidationHandler(
			invoke.NewSignatureValidationHandler(
				NewTxSubmitAndListenHandler(&txStatus),
			),
		),
	)
	result, err := client.channelClient.InvokeHandler(
		handlerChain,
		channel.Request{
			ChaincodeID:  chaincodeName,
			Fcn:          method,
			Args:         convertStringArray(args),
			TransientMap: convertStringMap(transientMap),
			IsInit:       isInit,
		},
		channel.WithRetry(retry.DefaultChannelOpts),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return client.signer, result.Payload, &txStatus, nil
}
