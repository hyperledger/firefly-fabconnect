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

package fabric

import (
	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest/identity"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel/invoke"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	fabcontext "github.com/hyperledger/fabric-sdk-go/pkg/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite/bccsp/sw"
	fabImpl "github.com/hyperledger/fabric-sdk-go/pkg/fab"
	eventsClient "github.com/hyperledger/fabric-sdk-go/pkg/fab/events/client"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/deliverclient"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/deliverclient/seek"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/service/blockfilter/headertypefilter"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/keyvaluestore"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	mspImpl "github.com/hyperledger/fabric-sdk-go/pkg/msp"
	mspApi "github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
	log "github.com/sirupsen/logrus"
)

type RPCClient interface {
	Init(channelId, signer, chaincodeName, method string, args []string) (*TxReceipt, error)
	Invoke(channelId, signer, chaincodeName, method string, args []string) (*TxReceipt, error)
	Query(channelId, signer, chaincodeName, method string, args []string) (*channel.Response, error)
	QueryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error)
	SubscribeEvent(channelId, signer string, since uint64) (fab.Registration, <-chan *fab.BlockEvent, fab.EventService, error)
	Close()
}

type clientWrapper struct {
	channelClient  *channel.Client
	ledgerClient   *ledger.Client
	channelService fab.ChannelService
	signer         *msp.IdentityIdentifier
}

type rpcWrapper struct {
	txTimeout         int
	sdk               *fabsdk.FabricSDK
	cryptoSuiteConfig core.CryptoSuiteConfig
	identityConfig    msp.IdentityConfig
	userStore         msp.UserStore
	identityMgr       msp.IdentityManager
	caClient          mspApi.CAClient
	// one channel client per channel ID, per signer ID
	channelClients map[string](map[string]*clientWrapper)
}

func RPCConnect(c conf.RPCConf, txTimeout int) (RPCClient, identity.IdentityClient, error) {
	configProvider := config.FromFile(c.ConfigPath)
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to initialize a new SDK instance. %s", err)
	}
	configBackend, _ := configProvider()
	endpointConfig, err := fabImpl.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to read config: %s", err)
	}
	cryptoConfig := cryptosuite.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to load crypto suite configurations: %s", err)
	}
	identityConfig, err := mspImpl.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to load identity configurations: %s", err)
	}
	clientConfig := identityConfig.Client()
	if clientConfig.CredentialStore.Path == "" {
		return nil, nil, errors.Errorf("User credentials store path is empty")
	}
	store, err := keyvaluestore.New(&keyvaluestore.FileKeyValueStoreOptions{
		Path: clientConfig.CredentialStore.Path,
	})
	if err != nil {
		return nil, nil, errors.Errorf("Key-value store creation failed. Path: %s", clientConfig.CredentialStore.Path)
	}
	userStore, err := mspImpl.NewCertFileUserStore1(store)
	if err != nil {
		return nil, nil, errors.Errorf("User credentials store creation failed. Path: %s", err)
	}
	cs, err := sw.GetSuiteByConfig(cryptoConfig)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to get suite by config: %s", err)
	}
	mgr, err := mspImpl.NewIdentityManager(clientConfig.Organization, userStore, cs, endpointConfig)
	if err != nil {
		return nil, nil, errors.Errorf("Identity manager creation failed. %s", err)
	}

	identityManagerProvider := &identityManagerProvider{
		identityManager: mgr,
	}
	ctxProvider := fabcontext.NewProvider(
		fabcontext.WithIdentityManagerProvider(identityManagerProvider),
		fabcontext.WithUserStore(userStore),
		fabcontext.WithCryptoSuite(cs),
		fabcontext.WithCryptoSuiteConfig(cryptoConfig),
		fabcontext.WithEndpointConfig(endpointConfig),
		fabcontext.WithIdentityConfig(identityConfig),
	)
	ctx := &fabcontext.Client{
		Providers: ctxProvider,
	}
	caClient, err := mspImpl.NewCAClient(clientConfig.Organization, ctx)
	if err != nil {
		return nil, nil, errors.Errorf("CA Client creation failed. %s", err)
	}

	log.Infof("New gRPC connection established")
	w := &rpcWrapper{
		sdk:               sdk,
		cryptoSuiteConfig: cryptoConfig,
		identityConfig:    identityConfig,
		userStore:         userStore,
		identityMgr:       mgr,
		caClient:          caClient,
		channelClients:    make(map[string]map[string]*clientWrapper),
		txTimeout:         txTimeout,
	}
	return w, w, nil
}

func (w *rpcWrapper) getChannelClient(channelId string, signer string) (*clientWrapper, error) {
	id, err := w.identityMgr.GetSigningIdentity(signer)
	if err == msp.ErrUserNotFound {
		return nil, errors.Errorf("Signer %s does not exist", signer)
	}
	if err != nil {
		return nil, errors.Errorf("Failed to retrieve signing identity: %s", err)
	}

	allClientsOfChannel := w.channelClients[channelId]
	if allClientsOfChannel == nil {
		w.channelClients[channelId] = make(map[string]*clientWrapper)
	}
	clientOfUser := w.channelClients[channelId][id.Identifier().ID]
	if clientOfUser == nil {
		channelContext := w.sdk.ChannelContext(channelId, fabsdk.WithOrg(id.Identifier().MSPID), fabsdk.WithUser(id.Identifier().ID))
		cClient, err := channel.New(channelContext)
		if err != nil {
			return nil, err
		}
		lClient, err := ledger.New(channelContext)
		if err != nil {
			return nil, err
		}
		channelProvider, err := channelContext()
		if err != nil {
			return nil, err
		}
		channelService := channelProvider.ChannelService()
		if err != nil {
			return nil, err
		}
		newWrapper := &clientWrapper{
			channelClient:  cClient,
			ledgerClient:   lClient,
			channelService: channelService,
			signer:         id.Identifier(),
		}
		w.channelClients[channelId][id.Identifier().ID] = newWrapper
		clientOfUser = newWrapper
	}
	return clientOfUser, nil
}

func (w *rpcWrapper) Init(channelId, signer, chaincodeName, method string, args []string) (*TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	signerID, result, txStatus, err := w.sendTransaction(channelId, signer, chaincodeName, method, args, true)
	if err != nil {
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return newReceipt(result, txStatus, signerID), err
}

func (w *rpcWrapper) Invoke(channelId, signer, chaincodeName, method string, args []string) (*TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	signerID, result, txStatus, err := w.sendTransaction(channelId, signer, chaincodeName, method, args, false)
	if err != nil {
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return newReceipt(result, txStatus, signerID), err
}

func (w *rpcWrapper) Query(channelId, signer, chaincodeName, method string, args []string) (*channel.Response, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)

	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
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
		return nil, err
	}

	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return &result, nil
}

func (w *rpcWrapper) QueryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error) {
	log.Tracef("RPC [%s] --> ChainInfo", channelId)

	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		return nil, errors.Errorf("Failed to get channel client. %s", err)
	}

	result, err := client.ledgerClient.QueryInfo()
	if err != nil {
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

// The returned registration must be closed when done
func (w *rpcWrapper) SubscribeEvent(channelId, signer string, since uint64) (fab.Registration, <-chan *fab.BlockEvent, fab.EventService, error) {
	client, err := w.getChannelClient(channelId, signer)
	if err != nil {
		return nil, nil, nil, errors.Errorf("Failed to get channel client. %s", err)
	}

	var eventService fab.EventService
	if since == 0 {
		// default is the latest block
		eventService, err = client.channelService.EventService(
			eventsClient.WithBlockEvents(),
		)
	} else {
		eventService, err = client.channelService.EventService(
			eventsClient.WithBlockEvents(),
			deliverclient.WithSeekType(seek.FromBlock),
			deliverclient.WithBlockNum(since),
		)
	}
	if err != nil {
		log.Errorf("Failed to create event service. %s", err)
		return nil, nil, nil, errors.Errorf("Failed to create event service. %s", err)
	}
	reg, notifier, err := eventService.RegisterBlockEvent(headertypefilter.New(cb.HeaderType_ENDORSER_TRANSACTION))
	if err != nil {
		return nil, nil, nil, errors.Errorf("Failed to subscribe to block events. %s", err)
	}
	log.Infof("Subscribed to events in channel %s from block %d (0 means newest)", channelId, since)
	log.Debugf("event service used: %s", eventService)
	return reg, notifier, eventService, nil
}

func (w *rpcWrapper) sendTransaction(channelId, signer, chaincodeName, method string, args []string, isInit bool) (*msp.IdentityIdentifier, *channel.Response, *fab.TxStatusEvent, error) {
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
	if err != nil {
		return nil, nil, nil, err
	}
	return client.signer, &result, &txStatus, nil
}

func (w *rpcWrapper) Close() {
	w.sdk.Close()
}

func convert(args []string) [][]byte {
	result := [][]byte{}
	for _, v := range args {
		result = append(result, []byte(v))
	}
	return result
}

func newReceipt(response *channel.Response, status *fab.TxStatusEvent, signerID *msp.IdentityIdentifier) *TxReceipt {
	return &TxReceipt{
		SignerMSP:     signerID.MSPID,
		Signer:        signerID.ID,
		TransactionID: string(response.TransactionID),
		Status:        response.TxValidationCode,
		BlockNumber:   status.BlockNumber,
		SourcePeer:    status.SourceURL,
	}
}
