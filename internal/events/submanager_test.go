// Copyright 2021 Kaleido

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
	"io/ioutil"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/events/api"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/test"
	"github.com/hyperledger/firefly-fabconnect/internal/kvstore"
	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

func TestInitLevelDBSuccess(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)

	router := &httprouter.Router{}
	svr := httptest.NewServer(router)
	defer svr.Close()

	sm := newTestSubscriptionManager()
	sm.config.LevelDB.Path = path.Join(dir, "db")
	err := sm.Init()
	assert.Equal(nil, err)
	sm.Close()
}

func TestInitLevelDBFail(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)
	_ = ioutil.WriteFile(path.Join(dir, "db"), []byte("I am not a directory"), 0644)
	sm := newTestSubscriptionManager()
	sm.config.LevelDB.Path = path.Join(dir, "db")
	err := sm.Init()
	assert.Regexp("not a directory", err.Error())
	sm.Close()
}

func TestActionAndSubscriptionLifecyle(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	subscriptionName := "testSub"
	defer cleanup(t, dir)
	sm := newTestSubscriptionManager()
	sm.config.LevelDB.Path = path.Join(dir, "db")
	err := sm.Init()
	assert.NoError(err)
	defer sm.db.Close()

	assert.Equal([]*api.SubscriptionInfo{}, sm.getSubscriptions())
	assert.Equal([]*StreamInfo{}, sm.getStreams())

	stream := &StreamInfo{
		Type:    "webhook",
		Webhook: &webhookActionInfo{URL: "http://test.invalid"},
	}
	err = sm.addStream(stream)
	assert.NoError(err)

	sub := &api.SubscriptionInfo{
		Name:   subscriptionName,
		Stream: stream.ID,
	}
	err = sm.addSubscription(sub)
	assert.NoError(err)

	assert.Equal([]*api.SubscriptionInfo{sub}, sm.getSubscriptions())
	assert.Equal([]*StreamInfo{stream}, sm.getStreams())

	retSub, _ := sm.subscriptionByID(sub.ID)
	assert.Equal(sub, retSub.info)
	retStream, _ := sm.streamByID(stream.ID)
	assert.Equal(stream, retStream.spec)

	assert.Nil(sm.subscriptionByID(stream.ID))
	assert.Nil(sm.streamByID(sub.ID))

	err = sm.suspendStream(retStream)
	assert.NoError(err)

	err = sm.suspendStream(retStream)
	assert.NoError(err)

	for {
		// Incase the suspend takes a little time
		if err = sm.resumeStream(retStream); err == nil {
			break
		} else {
			time.Sleep(1 * time.Millisecond)
		}
	}

	err = sm.resumeStream(retStream)
	assert.EqualError(err, "Event processor is already active. Suspending:false")

	// Reload
	sm.Close()

	sm = newTestSubscriptionManager()
	sm.config.LevelDB.Path = path.Join(dir, "db")
	err = sm.Init()
	assert.NoError(err)

	assert.Equal(1, len(sm.streams))
	assert.Equal(1, len(sm.subscriptions))

	reloadedSub, err := sm.subscriptionByID(retSub.info.ID)
	assert.NoError(err)
	err = sm.resetSubscription(reloadedSub, "0")
	assert.NoError(err)

	err = sm.deleteSubscription(reloadedSub)
	assert.NoError(err)

	reloadedStream, err := sm.streamByID(retStream.spec.ID)
	assert.NoError(err)
	err = sm.deleteStream(reloadedStream)
	assert.NoError(err)

	sm.Close()
}

func TestActionChildCleanup(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)
	sm := newTestSubscriptionManager()
	sm.rpc = test.MockRPCClient("")
	sm.db = kvstore.NewLDBKeyValueStore(path.Join(dir, "db"))
	_ = sm.db.Init()
	defer sm.db.Close()

	stream := &StreamInfo{
		Type:    "webhook",
		Webhook: &webhookActionInfo{URL: "http://test.invalid"},
	}
	err := sm.addStream(stream)
	assert.NoError(err)

	sub := &api.SubscriptionInfo{
		Name:   "testSub",
		Stream: stream.ID,
	}
	err = sm.addSubscription(sub)
	assert.NoError(err)
	err = sm.deleteStream(sm.streams[stream.ID])
	assert.NoError(err)

	assert.Equal([]*eventsapi.SubscriptionInfo{}, sm.getSubscriptions())
	assert.Equal([]*StreamInfo{}, sm.getStreams())

	sm.Close()
}

func TestStreamAndSubscriptionErrors(t *testing.T) {
	assert := assert.New(t)
	dir := tempdir(t)
	defer cleanup(t, dir)
	sm := newTestSubscriptionManager()
	sm.rpc = test.MockRPCClient("")
	sm.db = kvstore.NewLDBKeyValueStore(path.Join(dir, "db"))
	_ = sm.db.Init()
	defer sm.db.Close()

	assert.Equal([]*eventsapi.SubscriptionInfo{}, sm.getSubscriptions())
	assert.Equal([]*StreamInfo{}, sm.getStreams())

	stream := &StreamInfo{
		Type:    "webhook",
		Webhook: &webhookActionInfo{URL: "http://test.invalid"},
	}
	err := sm.addStream(stream)
	assert.NoError(err)

	sub := &api.SubscriptionInfo{
		Name:   "testSub",
		Stream: stream.ID,
	}
	err = sm.addSubscription(sub)
	assert.NoError(err)

	err = sm.resetSubscription(sm.subscriptions[sub.ID], "badness")
	assert.EqualError(err, "FromBlock cannot be parsed as a BigInt")

	sm.db.Close()
	err = sm.resetSubscription(sm.subscriptions[sub.ID], "0")
	assert.EqualError(err, "Failed to store subscription: leveldb: closed")

	sm.Close()
}

// func TestResetSubscriptionErrors(t *testing.T) {
// 	assert := assert.New(t)
// 	dir := tempdir(t)
// 	defer cleanup(t, dir)
// 	sm := newTestSubscriptionManager()
// 	sm.rpc = mockRPCClient("")
// 	db := &mockkvstore.KVStore{}
// 	db.On("Get", mock.Anything).Return(fmt.Errorf("pop"))
// 	db.On("Put", mock.Anything, mock.Anything).Return(fmt.Errorf("pop"))
// 	db.On("Delete", mock.Anything).Return(fmt.Errorf("pop"))
// 	sm.db = db
// 	defer sm.db.Close()

// 	ctx := context.Background()
// 	_, err := sm.AddStream(ctx, &StreamInfo{Type: "random"})
// 	assert.EqualError(err, "Unknown action type 'random'")
// 	_, err = sm.AddStream(ctx, &StreamInfo{
// 		Type:    "webhook",
// 		Webhook: &webhookActionInfo{URL: "http://test.invalid"},
// 	})
// 	assert.EqualError(err, "Failed to store stream: pop")
// 	sm.streams["teststream"] = newTestStream()
// 	err = sm.DeleteStream(ctx, "nope")
// 	assert.EqualError(err, "Stream with ID 'nope' not found")
// 	err = sm.SuspendStream(ctx, "nope")
// 	assert.EqualError(err, "Stream with ID 'nope' not found")
// 	err = sm.ResumeStream(ctx, "nope")
// 	assert.EqualError(err, "Stream with ID 'nope' not found")
// 	err = sm.DeleteStream(ctx, "teststream")
// 	assert.EqualError(err, "pop")

// 	_, err = sm.AddSubscription(ctx, nil, &ethbinding.ABIElementMarshaling{Name: "any"}, "nope", "", "")
// 	assert.EqualError(err, "Stream with ID 'nope' not found")
// 	_, err = sm.AddSubscription(ctx, nil, &ethbinding.ABIElementMarshaling{Name: "any"}, "teststream", "", "test")
// 	assert.EqualError(err, "Failed to store subscription: pop")
// 	_, err = sm.AddSubscription(ctx, nil, &ethbinding.ABIElementMarshaling{Name: "any"}, "teststream", "!bad integer", "")
// 	assert.EqualError(err, "FromBlock cannot be parsed as a BigInt")
// 	sm.subscriptions["testsub"] = &subscription{info: &SubscriptionInfo{}, rpc: sm.rpc}
// 	err = sm.ResetSubscription(ctx, "nope", "0")
// 	assert.EqualError(err, "Subscription with ID 'nope' not found")
// 	err = sm.DeleteSubscription(ctx, "nope")
// 	assert.EqualError(err, "Subscription with ID 'nope' not found")
// 	err = sm.DeleteSubscription(ctx, "testsub")
// 	assert.EqualError(err, "pop")
// }

// func TestRecoverErrors(t *testing.T) {
// 	assert := assert.New(t)
// 	dir := tempdir(t)
// 	defer cleanup(t, dir)
// 	sm := newTestSubscriptionManager()
// 	sm.db, _ = kvstore.NewLDBKeyValueStore(path.Join(dir, "db"))
// 	defer sm.db.Close()

// 	sm.db.Put(streamIDPrefix+"esid1", []byte(":bad json"))
// 	sm.db.Put(streamIDPrefix+"esid2", []byte("{}"))
// 	sm.db.Put(subIDPrefix+"subid1", []byte(":bad json"))
// 	sm.db.Put(subIDPrefix+"subid2", []byte("{}"))

// 	sm.recoverStreams()
// 	sm.recoverSubscriptions()

// 	assert.Equal(0, len(sm.streams))
// 	assert.Equal(0, len(sm.subscriptions))

// }
