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

package events

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/client"
	"github.com/hyperledger/firefly-fabconnect/internal/kvstore"
	restutil "github.com/hyperledger/firefly-fabconnect/internal/rest/utils"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/hyperledger/firefly-fabconnect/internal/ws"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

const (
	// SubPathPrefix is the path prefix for subscriptions
	SubPathPrefix = "/subscriptions"
	// StreamPathPrefix is the path prefix for event streams
	StreamPathPrefix   = "/eventstreams"
	subIDPrefix        = "sb-"
	streamIDPrefix     = "es-"
	checkpointIDPrefix = "cp-"
)

type ResetRequest struct {
	InitialBlock string `json:"initialBlock"`
}

// SubscriptionManager provides REST APIs for managing events
type SubscriptionManager interface {
	Init(mocked ...kvstore.KVStore) error
	AddStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*StreamInfo, *restutil.RestError)
	Streams(res http.ResponseWriter, req *http.Request, params httprouter.Params) []*StreamInfo
	StreamByID(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*StreamInfo, *restutil.RestError)
	UpdateStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*StreamInfo, *restutil.RestError)
	SuspendStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError)
	ResumeStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError)
	DeleteStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError)
	AddSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*eventsapi.SubscriptionInfo, *restutil.RestError)
	Subscriptions(res http.ResponseWriter, req *http.Request, params httprouter.Params) []*eventsapi.SubscriptionInfo
	SubscriptionByID(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*eventsapi.SubscriptionInfo, *restutil.RestError)
	ResetSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError)
	DeleteSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError)
	Close()
}

type subscriptionManager interface {
	getConfig() *conf.EventstreamConf
	streamByID(string) (*eventStream, error)
	subscriptionByID(string) (*subscription, error)
	subscriptionsForStream(string) []*subscription
	loadCheckpoint(string) (map[string]uint64, error)
	storeCheckpoint(string, map[string]uint64) error
}

type subscriptionMGR struct {
	config        *conf.EventstreamConf
	db            kvstore.KVStore
	rpc           client.RPCClient
	subscriptions map[string]*subscription
	streams       map[string]*eventStream
	closed        bool
	wsChannels    ws.WebSocketChannels
}

// NewSubscriptionManager constructor
func NewSubscriptionManager(config *conf.EventstreamConf, rpc client.RPCClient, wsChannels ws.WebSocketChannels) SubscriptionManager {
	sm := &subscriptionMGR{
		config:        config,
		rpc:           rpc,
		subscriptions: make(map[string]*subscription),
		streams:       make(map[string]*eventStream),
		wsChannels:    wsChannels,
	}
	if config.PollingIntervalSec <= 0 {
		config.PollingIntervalSec = 1
	}
	return sm
}

func (s *subscriptionMGR) Init(mocked ...kvstore.KVStore) error {
	if mocked != nil {
		// only used in tests to pass in a mocked impl
		s.db = mocked[0]
	} else {
		s.db = kvstore.NewLDBKeyValueStore(s.config.LevelDB.Path)
		err := s.db.Init()
		if err != nil {
			return errors.Errorf(errors.EventStreamsDBLoad, s.config.LevelDB.Path, err)
		}
	}
	s.recoverStreams()
	s.recoverSubscriptions()
	return nil
}

// StreamByID used externally to get serializable details
func (s *subscriptionMGR) StreamByID(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*StreamInfo, *restutil.RestError) {
	streamID := params.ByName("streamId")
	stream, err := s.streamByID(streamID)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	return stream.spec, nil
}

// Streams used externally to get list streams
func (s *subscriptionMGR) Streams(res http.ResponseWriter, req *http.Request, params httprouter.Params) []*StreamInfo {
	return s.getStreams()
}

// AddStream adds a new stream
func (s *subscriptionMGR) AddStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*StreamInfo, *restutil.RestError) {
	var spec StreamInfo
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf(errors.RESTGatewayEventStreamInvalid, err), 400)
	}
	st := strings.ToLower(spec.Type)
	if st != EventStreamTypeWebhook && st != EventStreamTypeWebsocket {
		return nil, restutil.NewRestError(fmt.Sprintf(errors.EventStreamsInvalidActionType, spec.Type), 400)
	}
	if st == EventStreamTypeWebhook {
		spec.Type = EventStreamTypeWebhook
		if err := validateWebhookConfig(spec.Webhook); err != nil {
			return nil, restutil.NewRestError(err.Error(), 400)
		}
	} else {
		spec.Type = EventStreamTypeWebsocket
		if err := validateWebsocketConfig(spec.WebSocket); err != nil {
			return nil, restutil.NewRestError(err.Error(), 400)
		}
	}
	if spec.ErrorHandling != "" {
		eh := strings.ToLower(spec.ErrorHandling)
		if eh != ErrorHandlingBlock && eh != ErrorHandlingSkip {
			return nil, restutil.NewRestError("Unknown errorHandling type. Must be an empty string, 'skip' or 'block'")
		} else {
			spec.ErrorHandling = eh
		}
	}
	if spec.Suspended != nil {
		return nil, restutil.NewRestError("Can not set 'suspended'")
	}

	if err := s.addStream(&spec); err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	return &spec, nil
}

// UpdateStream updates an existing stream
func (s *subscriptionMGR) UpdateStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*StreamInfo, *restutil.RestError) {
	streamID := params.ByName("streamId")
	stream, err := s.streamByID(streamID)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	var spec StreamInfo
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf(errors.RESTGatewayEventStreamInvalid, err), 400)
	}
	et := strings.ToLower(spec.Type)
	if et == EventStreamTypeWebhook {
		spec.Type = EventStreamTypeWebhook
		if _, err = url.Parse(spec.Webhook.URL); err != nil {
			return nil, restutil.NewRestError(errors.EventStreamsWebhookInvalidURL, 400)
		}
	} else if et == EventStreamTypeWebsocket {
		spec.Type = EventStreamTypeWebsocket
	}
	if spec.ErrorHandling != "" {
		eh := strings.ToLower(spec.ErrorHandling)
		if eh != ErrorHandlingBlock && eh != ErrorHandlingSkip {
			return nil, restutil.NewRestError("Unknown errorHandling type. Must be an empty string, 'skip' or 'block'", 400)
		} else {
			spec.ErrorHandling = eh
		}
	}
	if spec.Suspended != nil {
		return nil, restutil.NewRestError("Can not set 'suspended'")
	}
	updatedSpec, err := s.updateStream(stream, &spec)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	return updatedSpec, nil
}

// DeleteStream deletes a streamm
func (s *subscriptionMGR) DeleteStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError) {
	streamID := params.ByName("streamId")
	stream, err := s.streamByID(streamID)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	if err = s.deleteStream(stream); err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}

	result := map[string]string{}
	result["id"] = streamID
	result["deleted"] = "true"
	return &result, nil
}

// SuspendStream suspends a stream from firing
func (s *subscriptionMGR) SuspendStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError) {
	streamID := params.ByName("streamId")
	stream, err := s.streamByID(streamID)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	if err = s.suspendStream(stream); err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}

	result := map[string]string{}
	result["id"] = streamID
	result["suspended"] = "true"
	return &result, nil
}

// ResumeStream restarts a suspended stream
func (s *subscriptionMGR) ResumeStream(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError) {
	streamID := params.ByName("streamId")
	stream, err := s.streamByID(streamID)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	if err = s.resumeStream(stream); err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}

	result := map[string]string{}
	result["id"] = streamID
	result["resumed"] = "true"
	return &result, nil
}

// SubscriptionByID used externally to get serializable details
func (s *subscriptionMGR) SubscriptionByID(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*eventsapi.SubscriptionInfo, *restutil.RestError) {
	id := params.ByName("subscriptionId")
	sub, err := s.subscriptionByID(id)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	return sub.info, nil
}

// Subscriptions used externally to get list subscriptions
func (s *subscriptionMGR) Subscriptions(res http.ResponseWriter, req *http.Request, params httprouter.Params) []*eventsapi.SubscriptionInfo {
	return s.getSubscriptions()
}

// AddSubscription adds a new subscription
func (s *subscriptionMGR) AddSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*eventsapi.SubscriptionInfo, *restutil.RestError) {
	var spec eventsapi.SubscriptionInfo
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf(errors.RESTGatewaySubscriptionInvalid, err), 400)
	}
	if spec.ChannelId == "" {
		return nil, restutil.NewRestError(`Missing required parameter "channel"`, 400)
	}
	if spec.Stream == "" {
		return nil, restutil.NewRestError(`Missing required parameter "stream"`, 400)
	}
	if spec.Signer == "" {
		return nil, restutil.NewRestError(`Missing required parameter "signer"`, 400)
	}
	pt := spec.PayloadType
	if pt != "" && pt != eventsapi.EventPayloadType_String && pt != eventsapi.EventPayloadType_JSON {
		return nil, restutil.NewRestError(`Parameter "payloadType" must be an empty string, "string" or "json"`, 400)
	}
	bt := spec.Filter.BlockType
	if bt != "" && bt != eventsapi.BlockType_TX && bt != eventsapi.BlockType_Config {
		return nil, restutil.NewRestError(`Parameter "filter.blockType" must be an empty string, "tx" or "config"`, 400)
	}
	if err := validateFromBlock(spec.FromBlock); err != nil {
		return nil, restutil.NewRestError(err.Error(), 400)
	}

	if err, statusCode := s.addSubscription(&spec); err != nil {
		return nil, restutil.NewRestError(err.Error(), statusCode)
	}
	return &spec, nil
}

// ResetSubscription restarts the steam from the specified block
func (s *subscriptionMGR) ResetSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError) {
	id := params.ByName("subscriptionId")
	sub, err := s.subscriptionByID(id)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	var request ResetRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf("Failed to parse request body. %s", err), 400)
	}
	if err := validateFromBlock(request.InitialBlock); err != nil {
		return nil, restutil.NewRestError(err.Error(), 400)
	}
	err = s.resetSubscription(sub, request.InitialBlock)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	result := map[string]string{}
	result["id"] = sub.info.ID
	result["reset"] = "true"
	return &result, nil
}

// DeleteSubscription deletes a subscription
func (s *subscriptionMGR) DeleteSubscription(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*map[string]string, *restutil.RestError) {
	id := params.ByName("subscriptionId")
	sub, err := s.subscriptionByID(id)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 404)
	}
	err = s.deleteSubscription(sub)
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	result := map[string]string{}
	result["id"] = sub.info.ID
	result["deleted"] = "true"
	return &result, nil
}

func (s *subscriptionMGR) getConfig() *conf.EventstreamConf {
	return s.config
}

func (s *subscriptionMGR) getStreams() []*StreamInfo {
	l := make([]*StreamInfo, 0, len(s.subscriptions))
	for _, stream := range s.streams {
		l = append(l, stream.spec)
	}
	return l
}

// streamByID used internally to lookup full objects
func (s *subscriptionMGR) streamByID(id string) (*eventStream, error) {
	stream, exists := s.streams[id]
	if !exists {
		return nil, errors.Errorf(errors.EventStreamsStreamNotFound, id)
	}
	return stream, nil
}

func (s *subscriptionMGR) addStream(spec *StreamInfo) error {
	spec.ID = streamIDPrefix + utils.UUIDv4()
	spec.CreatedISO8601 = time.Now().UTC().Format(time.RFC3339)
	spec.Path = StreamPathPrefix + "/" + spec.ID
	stream, err := newEventStream(s, spec, s.wsChannels)
	if err != nil {
		return err
	}
	s.streams[stream.spec.ID] = stream
	return s.storeStream(stream.spec)
}

func (s *subscriptionMGR) updateStream(stream *eventStream, spec *StreamInfo) (*StreamInfo, error) {
	updatedSpec, err := stream.update(spec)
	if err != nil {
		return nil, err
	}
	if err := s.storeStream(updatedSpec); err != nil {
		return nil, err
	}
	return updatedSpec, nil
}

func (s *subscriptionMGR) deleteStream(stream *eventStream) error {
	// We have to clean up all the associated subs
	for _, sub := range s.subscriptions {
		if sub.info.Stream == stream.spec.ID {
			err := s.deleteSubscription(sub)
			if err != nil {
				log.Errorf("Failed to delete subscription from database. %s", err)
			}
		}
	}
	delete(s.streams, stream.spec.ID)
	stream.stop()
	if err := s.db.Delete(stream.spec.ID); err != nil {
		return err
	}
	s.deleteCheckpoint(stream.spec.ID)
	return nil
}

func (s *subscriptionMGR) suspendStream(stream *eventStream) error {
	stream.suspend()
	// Persist the state change
	if err := s.storeStream(stream.spec); err != nil {
		return err
	}
	return nil
}

func (s *subscriptionMGR) resumeStream(stream *eventStream) error {
	if err := stream.resume(); err != nil {
		return err
	}
	// Persist the state change
	if err := s.storeStream(stream.spec); err != nil {
		return err
	}
	return nil
}

func (s *subscriptionMGR) storeStream(spec *StreamInfo) error {
	infoBytes, _ := json.MarshalIndent(spec, "", "  ")
	if err := s.db.Put(spec.ID, infoBytes); err != nil {
		return errors.Errorf(errors.EventStreamsCreateStreamStoreFailed, err)
	}
	return nil
}

func (s *subscriptionMGR) getSubscriptions() []*eventsapi.SubscriptionInfo {
	l := make([]*eventsapi.SubscriptionInfo, 0, len(s.subscriptions))
	for _, sub := range s.subscriptions {
		l = append(l, sub.info)
	}
	return l
}

func (s *subscriptionMGR) addSubscription(spec *eventsapi.SubscriptionInfo) (error, int) {
	spec.TimeSorted = eventsapi.TimeSorted{
		CreatedISO8601: time.Now().UTC().Format(time.RFC3339),
	}
	spec.ID = subIDPrefix + utils.UUIDv4()
	spec.Path = SubPathPrefix + "/" + spec.ID
	// Check initial block number to subscribe from
	if spec.FromBlock == "" {
		// user did not set an initial block, default to newest
		spec.FromBlock = FromBlockNewest
	}
	if spec.Filter.BlockType == "" {
		spec.Filter.BlockType = eventsapi.BlockType_TX
	}
	if spec.Filter.EventFilter == "" && spec.Filter.ChaincodeId != "" {
		spec.Filter.EventFilter = ".*"
	}

	// A subscription is based on an event client and a registration. We must ensure that
	// on restart all subscriptions can be restored. We must avoid allowing different subscriptions
	// to register for the same parameters against the same event client, which would fail. NOTE that
	// upon restart, all subscriptions will have been set for the "fromBlock" to the latest checkpoint,
	// which would likely be the same across the board. As a result, "fromBlock" can not be a key segment
	// to index event clients.
	//
	// Because of the above, we require that a subscription must have a unique composite key made up of:
	// - channel ID
	// - chaincode ID (empty string if creating a block event listener)
	// - block type (endorser or config)
	// - event filter
	//
	// Note that, because of the way event clients are cached by fabric-sdk-go, subscriptions having the
	// same channel ID and chaincode ID, but different the event filter will share the same event client.
	// This means subsequent subscriptions will NOT get historical events, because the offset in the event
	// client will have been set to the latest block.
	subscriptionKey := calculateLookupKey(spec)
	_, err := s.db.Get(subscriptionKey)
	if err == nil {
		// a conflicting subscription already exists, return 400
		return errors.Error("A subscription with the same channel ID, chaincode ID, block type and event filter already exists"), 400
	}

	// Create it
	stream, err := s.streamByID(spec.Stream)
	if err != nil {
		return err, 500
	}
	sub, err := newSubscription(stream, s.rpc, spec)
	if err != nil {
		return err, 500
	}
	s.subscriptions[sub.info.ID] = sub
	return s.storeSubscription(spec, subscriptionKey), 200
}

func (s *subscriptionMGR) resetSubscription(sub *subscription, initialBlock string) error {
	// Re-set the inital block on the subscription and save it
	if initialBlock == "" || initialBlock == FromBlockNewest {
		sub.info.FromBlock = FromBlockNewest
	} else {
		_, err := strconv.ParseUint(initialBlock, 10, 64)
		if err != nil {
			return errors.Errorf(errors.EventStreamsSubscribeBadBlock)
		}

		sub.info.FromBlock = initialBlock
	}
	if err := s.storeSubscription(sub.info, calculateLookupKey(sub.info)); err != nil {
		return err
	}
	// Request a reset on the next poling cycle
	sub.requestReset()
	return nil
}

func (s *subscriptionMGR) deleteSubscription(sub *subscription) error {
	delete(s.subscriptions, sub.info.ID)
	sub.unsubscribe(true)
	if err := s.db.Delete(sub.info.ID); err != nil {
		return err
	}
	// also delete the lookup key entry
	subscriptionKey := calculateLookupKey(sub.info)
	if err := s.db.Delete(subscriptionKey); err != nil {
		return err
	}

	return nil
}

func (s *subscriptionMGR) storeSubscription(info *eventsapi.SubscriptionInfo, lookupKey string) error {
	infoBytes, _ := json.MarshalIndent(info, "", "  ")
	if err := s.db.Put(info.ID, infoBytes); err != nil {
		return errors.Errorf(errors.EventStreamsSubscribeStoreFailed, err)
	}
	if err := s.db.Put(lookupKey, []byte(info.ID)); err != nil {
		return errors.Errorf(errors.EventStreamsSubscribeLookupKeyStoreFailed, err)
	}
	return nil
}

func (s *subscriptionMGR) subscriptionsForStream(id string) []*subscription {
	subIDs := make([]*subscription, 0)
	for _, sub := range s.subscriptions {
		if sub.info.Stream == id {
			subIDs = append(subIDs, sub)
		}
	}
	return subIDs
}

// subscriptionByID used internally to lookup full objects
func (s *subscriptionMGR) subscriptionByID(id string) (*subscription, error) {
	sub, exists := s.subscriptions[id]
	if !exists {
		return nil, errors.Errorf(errors.EventStreamsSubscriptionNotFound, id)
	}
	return sub, nil
}

func (s *subscriptionMGR) loadCheckpoint(streamID string) (map[string]uint64, error) {
	cpID := checkpointIDPrefix + streamID
	b, err := s.db.Get(cpID)
	if err == leveldb.ErrNotFound {
		return make(map[string]uint64), nil
	} else if err != nil {
		return nil, err
	}
	log.Debugf("Loaded checkpoint %s: %s", cpID, string(b))
	var checkpoint map[string]uint64
	err = json.Unmarshal(b, &checkpoint)
	if err != nil {
		return nil, err
	}
	return checkpoint, nil
}

func (s *subscriptionMGR) storeCheckpoint(streamID string, checkpoint map[string]uint64) error {
	cpID := checkpointIDPrefix + streamID
	b, _ := json.MarshalIndent(&checkpoint, "", "  ")
	log.Debugf("Storing checkpoint %s: %s", cpID, string(b))
	return s.db.Put(cpID, b)
}

func (s *subscriptionMGR) deleteCheckpoint(streamID string) {
	cpID := checkpointIDPrefix + streamID
	err := s.db.Delete(cpID)
	if err != nil {
		log.Errorf("Failed to delete checkpoint from database. %s", err)
	}
}

func (s *subscriptionMGR) recoverStreams() {
	// Recover all the streams
	iStream := s.db.NewIterator()
	defer iStream.Release()
	for iStream.Next() {
		k := iStream.Key()
		if strings.HasPrefix(k, streamIDPrefix) {
			var streamInfo StreamInfo
			err := json.Unmarshal(iStream.Value(), &streamInfo)
			if err != nil {
				log.Errorf("Failed to recover stream '%s': %s", string(iStream.Value()), err)
				continue
			}
			if streamInfo.Suspended == nil {
				streamInfo.Suspended = &falseValue
			}
			if streamInfo.Timestamps == nil {
				streamInfo.Timestamps = &falseValue
			}
			if streamInfo.Webhook != nil && streamInfo.Webhook.TLSkipHostVerify == nil {
				streamInfo.Webhook.TLSkipHostVerify = &falseValue
			}
			stream, err := newEventStream(s, &streamInfo, s.wsChannels)
			if err != nil {
				log.Errorf("Failed to recover stream '%s': %s", streamInfo.ID, err)
			} else {
				s.streams[streamInfo.ID] = stream
			}
		}
	}
}

func (s *subscriptionMGR) recoverSubscriptions() {
	// Recover all the subscriptions
	iSub := s.db.NewIterator()
	defer iSub.Release()
	for iSub.Next() {
		k := iSub.Key()
		if strings.HasPrefix(k, subIDPrefix) {
			var subInfo eventsapi.SubscriptionInfo
			err := json.Unmarshal(iSub.Value(), &subInfo)
			if err != nil {
				log.Errorf("Failed to recover subscription '%s': %s", string(iSub.Value()), err)
				continue
			}
			stream, err := s.streamByID(subInfo.Stream)
			if err == nil {
				sub, err := restoreSubscription(stream, s.rpc, &subInfo)
				if err == nil {
					s.subscriptions[subInfo.ID] = sub
				}
			} else {
				log.Errorf("Failed to recover subscription '%s': %s", subInfo.ID, err)
			}
		}
	}
}

func (s *subscriptionMGR) Close() {
	log.Infof("Event stream subscription manager shutting down")
	for _, stream := range s.streams {
		stream.stop()
	}
	for _, sub := range s.subscriptions {
		sub.close()
	}
	if !s.closed && s.db != nil {
		s.db.Close()
	}
	s.closed = true
}

func validateFromBlock(fromBlock string) error {
	// from block property must be one of:
	// - empty string (newest)
	// - "newest"
	// - "<a valid number>"
	if fromBlock != "" && fromBlock != FromBlockNewest {
		if _, err := strconv.Atoi(fromBlock); err != nil {
			return fmt.Errorf("Invalid initial block: must be an integer, an empty string or 'newest'")
		}
	}
	return nil
}

func calculateLookupKey(spec *eventsapi.SubscriptionInfo) string {
	compositeKey := fmt.Sprintf("%s-%s-%s-%s", spec.ChannelId, spec.Filter.ChaincodeId, spec.Filter.BlockType, spec.Filter.EventFilter)
	hashKey := sha256.Sum256([]byte(compositeKey))
	subscriptionKey := fmt.Sprintf("sub-idx-%x", hashKey)
	return subscriptionKey
}
