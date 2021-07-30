// Copyright 2019 Kaleido
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

package events

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/fabric"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	log "github.com/sirupsen/logrus"
)

// persistedFilter is the part of the filter we record to storage
// BlockType:   optional. only notify on blocks of a specific type
//              types are defined in github.com/hyperledger/fabric-protos-go/common:
//              HeaderType_CONFIG, HeaderType_CONFIG_UPDATE, HeaderType_MESSAGE
// ChaincodeId: optional, only notify on blocks containing events for chaincode Id
// Filter:      optional. regexp applied to the event name. can be used independent of Chaincode ID
// FromBlock:   optional. "newest", "oldest", a number. default is "newest"
type persistedFilter struct {
	BlockType   string `json:"blockType,omitempty"`
	ChaincodeId string `json:"chaincodeId,omitempty"`
	Filter      string `json:"filter,omitempty"`
	FromBlock   uint64 `json:"fromBlock,omitempty"`
	ToBlock     uint64 `json:"toBlock,omitempty"`
}

// SubscriptionInfo is the persisted data for the subscription
type SubscriptionInfo struct {
	TimeSorted
	ID        string          `json:"id,omitempty"`
	ChannelId string          `json:"channel,omitempty"`
	Path      string          `json:"path"`
	Summary   string          `json:"-"`      // System generated name for the subscription
	Name      string          `json:"name"`   // User provided name for the subscription, set to Summary if missing
	Stream    string          `json:"stream"` // the event stream this subscription is associated under
	Signer    string          `json:"signer"`
	FromBlock string          `json:"fromBlock,omitempty"`
	Filter    persistedFilter `json:"filter"`
}

// subscription is the runtime that manages the subscription
type subscription struct {
	info           *SubscriptionInfo
	client         fabric.RPCClient
	ep             *evtProcessor
	registration   fab.Registration
	notifier       <-chan *fab.BlockEvent
	eventService   fab.EventService
	stop           <-chan bool
	filterStale    bool
	deleting       bool
	resetRequested bool
}

func newSubscription(sm subscriptionManager, rpc fabric.RPCClient, i *SubscriptionInfo) (*subscription, error) {
	stream, err := sm.streamByID(i.Stream)
	if err != nil {
		return nil, err
	}
	s := &subscription{
		info:        i,
		client:      rpc,
		ep:          newEvtProcessor(i.ID, stream),
		stop:        make(<-chan bool),
		filterStale: true,
	}
	i.Summary = fmt.Sprintf(`FromBlock=%d,Chaincode=%s,Filter=%s`, i.Filter.FromBlock, i.Filter.ChaincodeId, i.Filter.Filter)
	// If a name was not provided by the end user, set it to the system generated summary
	if i.Name == "" {
		log.Debugf("No name provided for subscription, using auto-generated ID:%s", i.ID)
		i.Name = i.ID
	}
	log.Infof("Created subscription ID:%s Chaincode: %s name:%s", i.ID, i.Filter.ChaincodeId, i.Name)
	return s, nil
}

// GetID returns the ID (for sorting)
func (info *SubscriptionInfo) GetID() string {
	return info.ID
}

func restoreSubscription(sm subscriptionManager, rpc fabric.RPCClient, i *SubscriptionInfo) (*subscription, error) {
	if i.GetID() == "" {
		return nil, errors.Errorf(errors.EventStreamsNoID)
	}
	stream, err := sm.streamByID(i.Stream)
	if err != nil {
		return nil, err
	}
	s := &subscription{
		client:      rpc,
		info:        i,
		ep:          newEvtProcessor(i.ID, stream),
		filterStale: true,
	}
	return s, nil
}

func (s *subscription) setInitialBlockHeight(ctx context.Context) (uint64, error) {
	if s.info.FromBlock != "" && s.info.FromBlock != FromBlockNewest {
		fromBlock, err := strconv.ParseUint(s.info.FromBlock, 10, 64)
		if err != nil {
			return 0, errors.Errorf(errors.EventStreamsSubscribeBadBlock)
		}
		return fromBlock, nil
	}
	result, err := s.client.QueryChainInfo(s.info.ChannelId, s.info.Signer)
	if err != nil {
		return 0, errors.Errorf(errors.RPCCallReturnedError, "QSCC GetChainInfo()", err)
	}
	i := result.BCI.Height
	s.ep.initBlockHWM(i)
	log.Infof("%s: initial block height for subscription (latest block): %d", s.info.ID, i)
	return i, nil
}

func (s *subscription) setCheckpointBlockHeight(i uint64) {
	s.ep.initBlockHWM(i)
	log.Infof("%s: checkpoint restored block height for subscription: %d", s.info.ID, i)
}

func (s *subscription) restartFilter(ctx context.Context, since uint64) error {
	reg, notifier, eventService, err := s.client.SubscribeEvent(s.info.ChannelId, s.info.Signer, since)
	if err != nil {
		return errors.Errorf(errors.RPCCallReturnedError, "SubscribeEvent", err)
	}
	s.registration = reg
	s.notifier = notifier
	s.eventService = eventService
	s.markFilterStale(false)

	// launch the events relay from the events pipe coming from the node to the batch queue
	go s.processNewEvents()

	log.Infof("%s: created filter from block %d: %+v", s.info.ID, since, s.info.Filter)
	return err
}

func (s *subscription) processNewEvents() {
	defer s.eventService.Unregister(s.registration)
	for {
		select {
		case blockEvent, ok := <-s.notifier:
			if !ok {
				log.Errorf("Unexpected close event from channel %s for subscription %s", s.info.ChannelId, s.info.ID)
				return
			}
			events := fabric.GetEvents(blockEvent.Block)
			for _, event := range events {
				if err := s.ep.processEventEntry(s.info.ID, event); err != nil {
					log.Errorf("Failed to process event: %s", err)
				}
			}
		case stopMsg, ok := <-s.stop:
			if ok && stopMsg {
				log.Infof("Event channel for subscription %s stopped", s.info.ID)
			}
		}
	}
}

func (s *subscription) unsubscribe(deleting bool) {
	log.Infof("%s: Unsubscribing existing filter (deleting=%t)", s.info.ID, deleting)
	s.deleting = deleting
	s.resetRequested = false
	s.markFilterStale(true)
}

func (s *subscription) requestReset() {
	// We simply set a flag, which is picked up by the event stream thread on the next polling cycle
	// and results in an unsubscribe/subscribe cycle.
	log.Infof("%s: Requested reset from block '%s'", s.info.ID, s.info.FromBlock)
	s.resetRequested = true
}

func (s *subscription) blockHWM() uint64 {
	return s.ep.getBlockHWM()
}

func (s *subscription) markFilterStale(newFilterStale bool) {
	log.Debugf("%s: Marking filter stale=%t, current sub filter stale=%t", s.info.ID, newFilterStale, s.filterStale)
	// If unsubscribe is called multiple times, we might not have a filter
	if newFilterStale && !s.filterStale {
		s.eventService.Unregister(s.registration)
		// We treat error as informational here - the filter might already not be valid (if the node restarted)
		log.Infof("%s: Uninstalled subscription by unregistering", s.info.ID)
	}
	s.filterStale = newFilterStale
}
