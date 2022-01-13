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
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/utils"
	log "github.com/sirupsen/logrus"
)

// the ccpClientWrapper implements the RPCClient interface using the fabric-sdk-go implementation
// based on the static network description provided via the CCP yaml
type ccpClientWrapper struct {
	channelClient   *channel.Client
	channelProvider context.ChannelProvider
	signer          *msp.IdentityIdentifier
}

// defined to allow mocking in tests
type channelCreator func(context.ChannelProvider) (*channel.Client, error)

type ccpRPCWrapper struct {
	txTimeout           int
	sdk                 *fabsdk.FabricSDK
	cryptoSuiteConfig   core.CryptoSuiteConfig
	userStore           msp.UserStore
	idClient            IdentityClient
	ledgerClientWrapper *ledgerClientWrapper
	eventClientWrapper  *eventClientWrapper
	channelCreator      channelCreator
	// one channel client per channel ID, per signer ID
	channelClients map[string](map[string]*ccpClientWrapper)
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
		sdk:                 ledgerClientWrapper.sdk,
		cryptoSuiteConfig:   cryptoConfig,
		userStore:           userStore,
		idClient:            idClient,
		ledgerClientWrapper: ledgerClientWrapper,
		eventClientWrapper:  eventClientWrapper,
		channelClients:      make(map[string]map[string]*ccpClientWrapper),
		channelCreator:      createChannelClient,
		txTimeout:           txTimeout,
	}
	return w, nil
}

func (w *ccpRPCWrapper) Invoke(channelId, signer, chaincodeName, method string, args []string, isInit bool) (*TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s:isInit=%t] --> %+v", channelId, chaincodeName, method, isInit, args)

	signerID, result, txStatus, err := w.sendTransaction(channelId, signer, chaincodeName, method, args, isInit)
	if err != nil {
		log.Errorf("Failed to send transaction [%s:%s:%s:isInit=%t]. %s", channelId, chaincodeName, method, isInit, err)
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s:isInit=%t] <-- %+v", channelId, chaincodeName, method, isInit, result)
	return newReceipt(result, txStatus, signerID), err
}

func (w *ccpRPCWrapper) Query(channelId, signer, chaincodeName, method string, args []string) ([]byte, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		log.Errorf("Failed to get channel client. %s", err)
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}

	result, err := client.channelClient.Query(
		channel.Request{
			ChaincodeID: chaincodeName,
			Fcn:         method,
			Args:        convert(args),
		},
		channel.WithRetry(retry.DefaultChannelOpts),
	)
	if err != nil {
		log.Errorf("Failed to send query [%s:%s:%s]. %s", channelId, chaincodeName, method, err)
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return result.Payload, nil
}

func (w *ccpRPCWrapper) QueryTransaction(channelId, signer, txId string) (map[string]interface{}, error) {
	log.Tracef("RPC [%s] --> QueryTransaction %s", channelId, txId)

	result, err := w.ledgerClientWrapper.queryTransaction(channelId, signer, txId)
	if err != nil {
		log.Errorf("Failed to query transaction on channel %s. %s", channelId, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

func (w *ccpRPCWrapper) QueryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error) {
	log.Tracef("RPC [%s] --> ChainInfo", channelId)

	result, err := w.ledgerClientWrapper.queryChainInfo(channelId, signer)
	if err != nil {
		log.Errorf("Failed to query chain info on channel %s. %s", channelId, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

func (w *ccpRPCWrapper) QueryBlock(channelId string, blockNumber uint64, signer string) (*utils.RawBlock, *utils.Block, error) {
	log.Tracef("RPC [%s] --> QueryBlock %v", channelId, blockNumber)

	rawblock, block, err := w.ledgerClientWrapper.queryBlock(channelId, blockNumber, signer)
	if err != nil {
		log.Errorf("Failed to query block %v on channel %s. %s", blockNumber, channelId, err)
		return nil, nil, err
	}

	log.Tracef("RPC [%s] <-- success", channelId)
	return rawblock, block, nil
}

// The returned registration must be closed when done
func (w *ccpRPCWrapper) SubscribeEvent(subInfo *eventsapi.SubscriptionInfo, since uint64) (*RegistrationWrapper, <-chan *fab.BlockEvent, <-chan *fab.CCEvent, error) {
	reg, blockEventCh, ccEventCh, err := w.eventClientWrapper.subscribeEvent(subInfo, since)
	if err != nil {
		log.Errorf("Failed to subscribe to event [%s:%s:%s]. %s", subInfo.Stream, subInfo.ChannelId, subInfo.Filter.ChaincodeId, err)
		return nil, nil, nil, err
	}
	return reg, blockEventCh, ccEventCh, nil
}

func (w *ccpRPCWrapper) Unregister(regWrapper *RegistrationWrapper) {
	regWrapper.eventClient.Unregister(regWrapper.registration)
}

func (w *ccpRPCWrapper) getChannelClient(channelId string, signer string) (*ccpClientWrapper, error) {
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

func createChannelClient(channelProvider context.ChannelProvider) (*channel.Client, error) {
	return channel.New(channelProvider)
}

func (w *ccpRPCWrapper) sendTransaction(channelId, signer, chaincodeName, method string, args []string, isInit bool) (*msp.IdentityIdentifier, []byte, *fab.TxStatusEvent, error) {
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
			ChaincodeID: chaincodeName,
			Fcn:         method,
			Args:        convert(args),
			IsInit:      isInit,
		},
		channel.WithRetry(retry.DefaultChannelOpts),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	return client.signer, result.Payload, &txStatus, nil
}

func convert(args []string) [][]byte {
	result := [][]byte{}
	for _, v := range args {
		result = append(result, []byte(v))
	}
	return result
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
