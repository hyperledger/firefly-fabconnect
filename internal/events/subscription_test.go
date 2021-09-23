// Copyright 2019 Kaleido

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
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/stretchr/testify/assert"
)

type mockSubMgr struct {
	stream        *eventStream
	subscription  *subscription
	err           error
	subscriptions []*subscription
}

func (m *mockSubMgr) getConfig() *conf.EventstreamConf {
	return &conf.EventstreamConf{}
}

func (m *mockSubMgr) streamByID(string) (*eventStream, error) {
	return m.stream, m.err
}

func (m *mockSubMgr) subscriptionByID(string) (*subscription, error) {
	return m.subscription, m.err
}

func (m *mockSubMgr) subscriptionsForStream(string) []*subscription {
	return m.subscriptions
}

func (m *mockSubMgr) loadCheckpoint(string) (map[string]uint64, error) { return nil, nil }

func (m *mockSubMgr) storeCheckpoint(string, map[string]uint64) error { return nil }

func testSubInfo(name string) *eventsapi.SubscriptionInfo {
	return &eventsapi.SubscriptionInfo{ID: "test", Stream: "streamID", Name: name}
}

func newTestStream(submgr subscriptionManager) *eventStream {
	a, _ := newEventStream(submgr, &StreamInfo{
		ID:   "123",
		Type: "WebHook",
		Webhook: &webhookActionInfo{
			URL: "http://hello.example.com/world",
		},
	}, nil)
	return a
}

func TestCreateWebhookSub(t *testing.T) {
	assert := assert.New(t)

	rpc := mockRPCClient("")
	m := &mockSubMgr{}
	m.stream = newTestStream(m)

	i := testSubInfo("glastonbury")
	s, err := newSubscription(m.stream, rpc, i)
	assert.NoError(err)
	assert.NotEmpty(s.info.ID)

	s1, err := restoreSubscription(m.stream, rpc, i)
	assert.NoError(err)

	assert.Equal(s.info.ID, s1.info.ID)
	assert.Equal("glastonbury", s1.info.Name)
	assert.Equal("", s.info.Filter.Filter)
	assert.Equal("FromBlock=,Chaincode=,Filter=", s1.info.Summary)
	assert.Equal("", s.info.Filter.BlockType)
}

func TestRestoreSubscriptionMissingID(t *testing.T) {
	assert := assert.New(t)
	m := &mockSubMgr{err: fmt.Errorf("nope")}
	testInfo := testSubInfo("")
	testInfo.ID = ""
	_, err := restoreSubscription(m.stream, nil, testInfo)
	assert.EqualError(err, "No ID")
}
