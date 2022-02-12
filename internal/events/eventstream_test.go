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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/kvstore"
	mockfabric "github.com/hyperledger/firefly-fabconnect/mocks/fabric/client"
	mockkvstore "github.com/hyperledger/firefly-fabconnect/mocks/kvstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestConstructorNoSpec(t *testing.T) {
	assert := assert.New(t)
	stream, err := newEventStream(newTestSubscriptionManager(), &StreamInfo{}, nil)
	defer stream.stop()
	assert.NoError(err)
}

func TestStopDuringTimeout(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize:      10,
			BatchTimeoutMS: 2000,
			Webhook:        &webhookActionInfo{},
		}, nil, 200)
	defer close(eventStream)
	defer svr.Close()

	stream.handleEvent(testEvent("sub1"))
	time.Sleep(10 * time.Millisecond)
	stream.stop()
	time.Sleep(10 * time.Millisecond)
	assert.True(stream.processorDone)
}

func TestBatchSizeCap(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 10000000,
			Webhook:   &webhookActionInfo{},
		}, nil, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	assert.Equal(uint64(MaxBatchSize), stream.spec.BatchSize)
	assert.Equal("", stream.spec.Name)
}

func TestStreamName(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			Name:    "testStream",
			Webhook: &webhookActionInfo{},
		}, nil, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	assert.Equal("testStream", stream.spec.Name)
}

func TestBlockingBehavior(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 1,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
			ErrorHandling:        ErrorHandlingBlock,
			BlockedRetryDelaySec: 1,
			Suspended:            &falseValue,
			Timestamps:           &falseValue,
		}, nil, 404)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	complete := false
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() { <-eventStream; wg.Done() }()
	stream.handleEvent(&eventData{
		event: &eventsapi.EventEntry{
			SubID: "sub1",
		},
		batchComplete: func(*eventsapi.EventEntry) { complete = true },
	})
	wg.Wait()
	time.Sleep(10 * time.Millisecond)
	assert.False(complete)
}

func TestSkippingBehavior(t *testing.T) {
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 1,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
			ErrorHandling:        ErrorHandlingSkip,
			BlockedRetryDelaySec: 1,
		}, nil, 404 /* fail the requests */)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	complete := false
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { <-eventStream; wg.Done() }()
	stream.handleEvent(&eventData{
		event: &eventsapi.EventEntry{
			SubID: "sub1",
		},
		batchComplete: func(*eventsapi.EventEntry) { complete = true },
	})
	wg.Wait()
	for !complete {
		time.Sleep(50 * time.Millisecond)
	}
	// reaching here despite the 404s means we passed
}

func TestBackoffRetry(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 1,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
			ErrorHandling:        ErrorHandlingBlock,
			RetryTimeoutSec:      1,
			BlockedRetryDelaySec: 1,
		}, nil, 404, 500, 503, 504, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()
	stream.initialRetryDelay = 1 * time.Millisecond
	stream.backoffFactor = 1.1

	complete := false
	thrown := false
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 5; i++ {
			<-eventStream
			thrown = true
		}
		wg.Done()
	}()
	stream.handleEvent(&eventData{
		event: &eventsapi.EventEntry{
			SubID: "sub1",
		},
		batchComplete: func(*eventsapi.EventEntry) { complete = true },
	})
	wg.Wait()
	assert.True(thrown)
	for !complete {
		time.Sleep(1 * time.Millisecond)
	}
}

func TestBlockedAddresses(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, nil, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	stream.allowPrivateIPs = false

	complete := false
	go func() { <-eventStream }()
	stream.handleEvent(&eventData{
		event: &eventsapi.EventEntry{
			SubID: "sub1",
		},
		batchComplete: func(*eventsapi.EventEntry) { complete = true },
	})
	time.Sleep(10 * time.Millisecond)
	assert.False(complete)
}

func TestBadDNSName(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingSkip,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, nil, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()
	stream.spec.Webhook.URL = "http://fail.invalid"

	called := false
	complete := false
	go func() { <-eventStream; called = true }()
	stream.handleEvent(&eventData{
		event: &eventsapi.EventEntry{
			SubID: "sub1",
		},
		batchComplete: func(*eventsapi.EventEntry) { complete = true },
	})
	for !complete {
		time.Sleep(1 * time.Millisecond)
	}
	assert.False(called)
}

func TestBatchTimeout(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize:      10,
			BatchTimeoutMS: 50,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	var e1s []*eventsapi.EventEntry
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e1s = <-eventStream
		wg.Done()
	}()
	for i := 0; i < 3; i++ {
		stream.handleEvent(testEvent(fmt.Sprintf("sub%d", i)))
	}
	wg.Wait()
	assert.Equal(3, len(e1s))

	var e2s, e3s []*eventsapi.EventEntry
	wg = sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e2s = <-eventStream
		e3s = <-eventStream
		wg.Done()
	}()
	for i := 0; i < 19; i++ {
		stream.handleEvent(testEvent(fmt.Sprintf("sub%d", i)))
	}
	wg.Wait()
	assert.Equal(10, len(e2s))
	assert.Equal(9, len(e3s))
	for i := 0; i < 10 && stream.inFlight > 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(uint64(0), stream.inFlight)

}

func TestBuildup(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, nil, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	assert.False(stream.isBlocked())

	// Hang the HTTP requests (no consumption from channel)
	for i := 0; i < 11; i++ {
		stream.handleEvent(testEvent(fmt.Sprintf("sub%d", i)))
	}

	for !stream.isBlocked() {
		time.Sleep(1 * time.Millisecond)
	}
	assert.True(stream.inFlight >= 10)

	for i := 0; i < 11; i++ {
		<-eventStream
	}
	for stream.isBlocked() {
		time.Sleep(1 * time.Millisecond)
	}
	assert.Equal(uint64(0), stream.inFlight)

}

func TestWebSocketUnconfigured(t *testing.T) {
	assert := assert.New(t)
	sm := NewSubscriptionManager(&conf.EventstreamConf{}, nil, nil).(*subscriptionMGR)
	err := sm.addStream(&StreamInfo{Type: "websocket"})
	assert.EqualError(err, "WebSocket listener not configured")
}

func TestProcessEventsEnd2EndWebhook(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 1,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
			Timestamps: &trueValue,
		}, db, 200)
	defer svr.Close()

	s := setupTestSubscription(sm, stream, "mySubName", "")
	assert.Equal("mySubName", s.Name)

	// We expect three events to be sent to the webhook
	// With the default batch size of 1, that means three separate requests
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		// the block event
		e1s := <-eventStream
		assert.Equal(1, len(e1s))
		assert.Equal(uint64(11), e1s[0].BlockNumber)
		// the chaincode event
		e2s := <-eventStream
		assert.Equal(1, len(e2s))
		assert.Equal(uint64(10), e2s[0].BlockNumber)
		assert.Equal(int64(1000000), e2s[0].Timestamp)
		wg.Done()
	}()
	wg.Wait()

	sub := sm.subscriptions[s.ID]
	err := sm.deleteSubscription(sub)
	assert.NoError(err)
	existingStream := sm.streams[stream.spec.ID]
	err = sm.deleteStream(existingStream)
	assert.NoError(err)
	sm.Close()
}

func TestProcessEventsEnd2EndCatchupWebhook(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 2,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
			Timestamps: &falseValue,
		}, db, 200)
	defer svr.Close()

	s := setupTestSubscription(sm, stream, "mySubName", "0")
	assert.Equal("mySubName", s.Name)

	// We expect three events to be sent to the webhook
	// With the default batch size of 1, that means three separate requests
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e1s := <-eventStream
		assert.Equal(2, len(e1s))
		assert.Equal(uint64(1), e1s[0].BlockNumber)
		assert.Equal(uint64(11), e1s[1].BlockNumber)
		wg.Done()
	}()
	wg.Wait()

	sub := sm.subscriptions[s.ID]
	err := sm.deleteSubscription(sub)
	assert.NoError(err)
	existingStream := sm.streams[stream.spec.ID]
	err = sm.deleteStream(existingStream)
	assert.NoError(err)
	sm.Close()
}

func TestProcessEventsEnd2EndWebSocket(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, mockWebSocket := newTestStreamForWebSocket(
		&StreamInfo{
			BatchSize:  1,
			Type:       "websocket",
			WebSocket:  &webSocketActionInfo{},
			Timestamps: &falseValue,
		}, db, 200)

	s := setupTestSubscription(sm, stream, "mySubName", "")
	assert.Equal("mySubName", s.Name)

	// We expect three events to be sent to the webhook
	// With the default batch size of 1, that means three separate requests
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e1s := (<-mockWebSocket.sender).([]*eventsapi.EventEntry)
		assert.Equal(1, len(e1s))
		assert.Equal(uint64(11), e1s[0].BlockNumber)
		mockWebSocket.receiver <- nil
		wg.Done()
	}()
	wg.Wait()

	sub := sm.subscriptions[s.ID]
	err := sm.deleteSubscription(sub)
	assert.NoError(err)
	existingStream := sm.streams[stream.spec.ID]
	err = sm.deleteStream(existingStream)
	assert.NoError(err)
	sm.Close()
}

func TestProcessEventsEnd2EndWithReset(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			BatchSize: 1,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
			Timestamps: &falseValue,
		}, db, 200)
	defer svr.Close()

	s := setupTestSubscription(sm, stream, "mySubName", "", true)
	assert.Equal("mySubName", s.Name)

	// We expect three events to be sent to the webhook
	// With the default batch size of 1, that means three separate requests
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e1s := <-eventStream
		assert.Equal(1, len(e1s))
		assert.Equal(uint64(11), e1s[0].BlockNumber)
		// the chaincode event
		e2s := <-eventStream
		assert.Equal(1, len(e2s))
		assert.Equal(uint64(10), e2s[0].BlockNumber)
		wg.Done()
	}()
	wg.Wait()

	sub := sm.subscriptions[s.ID]
	err := sm.resetSubscription(sub, "0")
	assert.NoError(err)

	wg = &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		e1s := <-eventStream
		assert.Equal(1, len(e1s))
		assert.Equal(uint64(11), e1s[0].BlockNumber)
		wg.Done()
	}()
	wg.Wait()

	err = sm.deleteSubscription(sub)
	assert.NoError(err)
	existingStream := sm.streams[stream.spec.ID]
	err = sm.deleteStream(existingStream)
	assert.NoError(err)
	sm.Close()
}

func TestInterruptWebSocketBroadcast(t *testing.T) {
	wsChannels := &mockWebSocket{
		sender:   make(chan interface{}),
		receiver: make(chan error),
	}
	es := &eventStream{
		wsChannels:      wsChannels,
		updateInterrupt: make(chan struct{}),
	}
	sio, _ := newWebSocketAction(es, &webSocketActionInfo{
		DistributionMode: "broadcast",
	})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		close(es.updateInterrupt)
		wg.Done()
	}()
	_ = sio.attemptBatch(0, 1, []*eventsapi.EventEntry{})
	wg.Wait()
}

func TestInterruptWebSocketSend(t *testing.T) {
	wsChannels := &mockWebSocket{
		sender:   make(chan interface{}),
		receiver: make(chan error),
	}
	es := &eventStream{
		wsChannels:      wsChannels,
		updateInterrupt: make(chan struct{}),
	}
	sio, _ := newWebSocketAction(es, &webSocketActionInfo{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		close(es.updateInterrupt)
		wg.Done()
	}()
	_ = sio.attemptBatch(0, 1, []*eventsapi.EventEntry{})
	wg.Wait()
}

func TestInterruptWebSocketReceive(t *testing.T) {
	wsChannels := &mockWebSocket{
		sender:    make(chan interface{}),
		broadcast: make(chan interface{}),
		receiver:  make(chan error),
		closing:   make(chan struct{}),
	}
	es := &eventStream{
		wsChannels:      wsChannels,
		updateInterrupt: make(chan struct{}),
	}
	sio, _ := newWebSocketAction(es, &webSocketActionInfo{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		<-wsChannels.sender
		close(es.updateInterrupt)
		wg.Done()
	}()
	_ = sio.attemptBatch(0, 1, []*eventsapi.EventEntry{})
	wg.Wait()
}

func TestPauseResumeAfterCheckpoint(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)
	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 2; i++ {
			<-eventStream
		}
		wg.Done()
	}()

	s := setupTestSubscription(sm, stream, "myTestSub", "")
	assert.Equal("myTestSub", s.Name)

	for {
		time.Sleep(1 * time.Millisecond)
		cp, err := sm.loadCheckpoint(stream.spec.ID)
		if err == nil {
			v, exists := cp[s.ID]
			t.Logf("Checkpoint? %t (%+v)", exists, v)
			if v == uint64(12) {
				break
			}
		}
	}
	stream.suspend()
	for !stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}

	// Restart from the checkpoint that was stored
	sub := sm.subscriptions[s.ID]
	sub.filterStale = true
	_ = stream.resume()
	for sub.filterStale {
		time.Sleep(1 * time.Millisecond)
	}
	wg.Wait()

	calls := sm.rpc.(*mockfabric.RPCClient).Calls
	assert.Equal(3, len(calls))
	since := calls[2].Arguments.Get(1)
	// the "since" would have been based on the stored checkpoint
	assert.Equal(uint64(12), since.(uint64))
}

func TestPauseResumeBeforeCheckpoint(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)
	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	stream.suspend()
	for !stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}

	s := setupTestSubscription(sm, stream, "", "")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 2; i++ {
			<-eventStream
		}
		wg.Done()
	}()

	sub := sm.subscriptions[s.ID]
	sub.filterStale = true
	_ = stream.resume()
	for sub.filterStale {
		time.Sleep(1 * time.Millisecond)
	}

	calls := sm.rpc.(*mockfabric.RPCClient).Calls
	assert.Equal(2, len(calls))
	since := calls[1].Arguments.Get(1)
	// the "since" would have been based on the block height
	// because no checkpoint was stored before the pause
	assert.Equal(uint64(10), since.(uint64))
	wg.Wait()
}

func TestMarkStaleOnError(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)
	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 2; i++ {
			<-eventStream
		}
		wg.Done()
	}()

	s := setupTestSubscription(sm, stream, "", "")
	sub := sm.subscriptions[s.ID]
	wg.Wait()

	stream.suspend()
	for !stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}
	assert.True(sub.filterStale)

	rpc := &mockfabric.RPCClient{}
	rpc.On("SubscribeEvent", mock.Anything, mock.Anything).Return(nil, nil, nil, fmt.Errorf("Failed to subscribe"))
	sub.client = rpc

	_ = stream.resume()
	for stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}
	for len(rpc.Calls) < 1 {
		time.Sleep(1 * time.Millisecond)
	}
	assert.True(sub.filterStale)

}

func TestStoreCheckpointLoadError(t *testing.T) {
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, nil, 200)
	mockKV := &mockkvstore.KVStore{}
	var emptyBytes []byte
	mockKV.On("Get", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "cp-")
	})).Return(emptyBytes, fmt.Errorf("pop"))
	sm.db = mockKV
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	stream.suspend()
	for !stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}
	_ = stream.resume()
	for stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}
}

func TestStoreCheckpointStoreError(t *testing.T) {
	mockKV := &mockkvstore.KVStore{}
	mockKV.On("Put", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "cp-")
	}), mock.Anything).Return(fmt.Errorf("pop"))
	mockKV.On("Get", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "cp-")
	})).Return([]byte("{}"), nil)
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, mockKV, 200)

	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for i := 0; i < 2; i++ {
			<-eventStream
		}
		wg.Done()
	}()
	setupTestSubscription(sm, stream, "", "")
	wg.Wait()

	stream.suspend()
	for !stream.pollerDone {
		time.Sleep(1 * time.Millisecond)
	}
}

func TestProcessBatchEmptyArray(t *testing.T) {
	mockKV := &mockkvstore.KVStore{}
	mockKV.On("Put", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "cp-")
	}), mock.Anything).Return(fmt.Errorf("pop"))
	mockKV.On("Get", mock.MatchedBy(func(key string) bool {
		return strings.HasPrefix(key, "cp-")
	})).Return([]byte("{}"), nil)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, mockKV, 200)
	defer close(eventStream)
	defer svr.Close()
	defer stream.stop()

	stream.processBatch(0, []*eventData{})
}

func TestUpdateStream(t *testing.T) {
	// The test performs the following steps:
	// * Create a stream with batch size 5
	// * Push 3 events
	// * Update the event stream and verify updated fields
	// * close event stream
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingSkip,
			BatchSize:     5,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer svr.Close()
	defer close(eventStream)
	defer stream.stop()

	for i := 0; i < 3; i++ {
		stream.handleEvent(testEvent(fmt.Sprintf("sub%d", i)))
	}
	headers := make(map[string]string)
	headers["test-h1"] = "val1"
	updateSpec := &StreamInfo{
		BatchSize:            4,
		BatchTimeoutMS:       10000,
		BlockedRetryDelaySec: 5,
		ErrorHandling:        ErrorHandlingBlock,
		Name:                 "new-name",
		Webhook: &webhookActionInfo{
			URL:               "http://foo.url",
			Headers:           headers,
			TLSkipHostVerify:  &trueValue,
			RequestTimeoutSec: 0,
		},
		Timestamps: &trueValue,
	}
	updatedStream, err := sm.updateStream(stream, updateSpec)
	assert.Equal("new-name", updatedStream.Name)
	assert.Equal(true, *updatedStream.Timestamps)
	assert.Equal(uint64(4), updatedStream.BatchSize)
	assert.Equal(uint64(10000), updatedStream.BatchTimeoutMS)
	assert.Equal(uint64(5), updatedStream.BlockedRetryDelaySec)
	assert.Equal(ErrorHandlingBlock, updatedStream.ErrorHandling)
	assert.Equal("http://foo.url", updatedStream.Webhook.URL)
	assert.Equal("val1", updatedStream.Webhook.Headers["test-h1"])

	assert.NoError(err)
}

func TestUpdateStreamWithDefaults(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			Suspended:     &falseValue,
			Timestamps:    &falseValue,
			ErrorHandling: ErrorHandlingSkip,
			BatchSize:     5,
			Webhook: &webhookActionInfo{
				URL:              "http://foo.url",
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer svr.Close()
	defer close(eventStream)
	defer stream.stop()

	for i := 0; i < 3; i++ {
		stream.handleEvent(testEvent(fmt.Sprintf("sub%d", i)))
	}
	headers := make(map[string]string)
	headers["test-h1"] = "val1"
	updateSpec := &StreamInfo{
		Webhook: &webhookActionInfo{
			Headers:          headers,
			TLSkipHostVerify: &trueValue,
		},
		Timestamps: &trueValue,
	}
	updatedStream, err := sm.updateStream(stream, updateSpec)
	assert.Equal(true, *updatedStream.Timestamps)
	assert.Equal(true, *updatedStream.Webhook.TLSkipHostVerify)
	assert.Equal(uint64(5), updatedStream.BatchSize)
	assert.Equal(ErrorHandlingSkip, updatedStream.ErrorHandling)
	assert.Contains(updatedStream.Webhook.URL, "http://127.0.0.1") // test that the URL has not been overriden
	assert.Equal("val1", updatedStream.Webhook.Headers["test-h1"])

	assert.NoError(err)
}

func TestUpdateStreamSwapType(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			BatchSize:     5,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer svr.Close()
	defer close(eventStream)
	defer stream.stop()

	updateSpec := &StreamInfo{
		Type: "websocket",
		WebSocket: &webSocketActionInfo{
			Topic: "test1",
		},
	}
	_, err := sm.updateStream(stream, updateSpec)
	assert.EqualError(err, "The type of an event stream cannot be changed")
}

func TestUpdateWebSocket(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	sm, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			BatchSize:     5,
			Type:          "websocket",
			Name:          "websocket-stream",
			WebSocket: &webSocketActionInfo{
				Topic: "test1",
			},
		}, db, 200)
	defer svr.Close()
	defer close(eventStream)
	defer stream.stop()

	updateSpec := &StreamInfo{
		WebSocket: &webSocketActionInfo{
			Topic: "test2",
		},
	}
	updatedStream, err := sm.updateStream(stream, updateSpec)
	assert.Equal("test2", updatedStream.WebSocket.Topic)
	assert.Equal("websocket-stream", updatedStream.Name)
	assert.NoError(err)
}

func TestWebSocketClientClosedOnSend(t *testing.T) {

	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			BatchSize:     5,
			Type:          "websocket",
			WebSocket: &webSocketActionInfo{
				Topic: "test1",
			},
		}, db, 200)
	defer svr.Close()
	defer close(eventStream)
	defer stream.stop()

	mws := stream.wsChannels.(*mockWebSocket)
	wsa := stream.action.(*webSocketAction)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		_ = wsa.attemptBatch(0, 0, []*eventsapi.EventEntry{})
		wg.Done()
	}()

	close(mws.closing)
	wg.Wait()

}

func TestWebSocketClientClosedOnReceive(t *testing.T) {

	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			BatchSize:     5,
			Type:          "websocket",
			WebSocket: &webSocketActionInfo{
				Topic: "test1",
			},
		}, db, 200)
	defer svr.Close()
	defer close(eventStream)
	defer stream.stop()

	mws := stream.wsChannels.(*mockWebSocket)
	wsa := stream.action.(*webSocketAction)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		_ = wsa.attemptBatch(0, 0, []*eventsapi.EventEntry{})
		wg.Done()
	}()

	<-mws.sender

	close(mws.closing)
	wg.Wait()

}

func TestUpdateStreamDuplicateCall(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	db := kvstore.NewLDBKeyValueStore(dir)
	_ = db.Init()
	_, stream, svr, _ := newTestStreamForBatching(
		&StreamInfo{
			ErrorHandling: ErrorHandlingBlock,
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, db, 200)
	defer svr.Close()

	err := stream.preUpdateStream()
	assert.NoError(err)

	err = stream.preUpdateStream()
	assert.Regexp("Update to event stream already in progress", err)
}
