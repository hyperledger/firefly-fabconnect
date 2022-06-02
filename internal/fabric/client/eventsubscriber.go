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

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/event"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/deliverclient/seek"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/events/service/blockfilter/headertypefilter"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	log "github.com/sirupsen/logrus"
)

// defined to allow mocking in tests
type eventClientCreator func(channelProvider context.ChannelProvider, opts ...event.ClientOption) (*event.Client, error)

type eventClientWrapper struct {
	// event client per channel per signer
	eventClients       map[string]map[string]*event.Client
	sdk                *fabsdk.FabricSDK
	idClient           IdentityClient
	eventClientCreator eventClientCreator
	mu                 sync.Mutex
}

func newEventClient(configProvider core.ConfigProvider, sdk *fabsdk.FabricSDK, idClient IdentityClient) *eventClientWrapper {
	w := &eventClientWrapper{
		sdk:                sdk,
		idClient:           idClient,
		eventClients:       make(map[string]map[string]*event.Client),
		eventClientCreator: createEventClient,
	}

	idClient.AddSignerUpdateListener(w)
	return w
}

func (e *eventClientWrapper) subscribeEvent(subInfo *eventsapi.SubscriptionInfo, since uint64) (*RegistrationWrapper, <-chan *fab.BlockEvent, <-chan *fab.CCEvent, error) {
	eventClient, err := e.getEventClient(subInfo.ChannelId, subInfo.Signer, since, subInfo.Filter.ChaincodeId)
	if err != nil {
		log.Errorf("Failed to get event client. %s", err)
		return nil, nil, nil, errors.Errorf("Failed to get event client. %s", err)
	}
	if subInfo.Filter.ChaincodeId != "" {
		reg, notifier, err := eventClient.RegisterChaincodeEvent(subInfo.Filter.ChaincodeId, subInfo.Filter.EventFilter)
		if err != nil {
			return nil, nil, nil, errors.Errorf("Failed to subscribe to chaincode %s events. %s", subInfo.Filter.ChaincodeId, err)
		}
		log.Infof("Subscribed to events in channel %s chaincode %s from block %d", subInfo.ChannelId, subInfo.Filter.ChaincodeId, since)
		regWrapper := &RegistrationWrapper{
			registration: reg,
			eventClient:  eventClient,
		}
		return regWrapper, nil, notifier, nil
	} else {
		blockType := subInfo.Filter.BlockType
		var blockfilter fab.BlockFilter
		if blockType == eventsapi.BlockType_TX {
			blockfilter = headertypefilter.New(common.HeaderType_ENDORSER_TRANSACTION)
		} else if blockType == eventsapi.BlockType_Config {
			blockfilter = headertypefilter.New(common.HeaderType_CONFIG, common.HeaderType_CONFIG_UPDATE)
		}

		reg, notifier, err := eventClient.RegisterBlockEvent(blockfilter)
		if err != nil {
			return nil, nil, nil, errors.Errorf("Failed to subscribe to block events. %s", err)
		}
		log.Infof("Subscribed to events in channel %s from block %d", subInfo.ChannelId, since)
		regWrapper := &RegistrationWrapper{
			registration: reg,
			eventClient:  eventClient,
		}
		return regWrapper, notifier, nil, nil
	}
}

func (e *eventClientWrapper) getEventClient(channelId, signer string, since uint64, chaincodeId string) (eventClient *event.Client, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	eventClientsForSigner := e.eventClients[signer]
	if eventClientsForSigner == nil {
		eventClientsForSigner = make(map[string]*event.Client)
		e.eventClients[signer] = eventClientsForSigner
	}
	key := eventsapi.GetKeyForEventClient(channelId, chaincodeId)
	eventClient = eventClientsForSigner[key]
	if eventClient == nil {
		eventOpts := []event.ClientOption{
			event.WithBlockEvents(),
			event.WithSeekType(seek.FromBlock),
			event.WithBlockNum(since),
		}
		if chaincodeId != "" {
			eventOpts = append(eventOpts, event.WithChaincodeID(chaincodeId))
		}
		channelProvider := e.sdk.ChannelContext(channelId, fabsdk.WithOrg(e.idClient.GetClientOrg()), fabsdk.WithUser(signer))
		eventClient, err = e.eventClientCreator(channelProvider, eventOpts...)
		if err != nil {
			return nil, err
		}
		eventClientsForSigner[key] = eventClient
	}
	return eventClient, nil
}

func (e *eventClientWrapper) SignerUpdated(signer string) {
	e.mu.Lock()
	e.eventClients[signer] = nil
	e.mu.Unlock()
}

func createEventClient(channelProvider context.ChannelProvider, eventOpts ...event.ClientOption) (*event.Client, error) {
	return event.New(channelProvider, eventOpts...)
}
