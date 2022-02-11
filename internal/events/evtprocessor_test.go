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
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/events/api"

	"github.com/stretchr/testify/assert"
)

func TestEventPayloadUnmarshaling(t *testing.T) {
	assert := assert.New(t)
	_, stream, svr, eventStream := newTestStreamForBatching(
		&StreamInfo{
			Name: "testStream",
			Webhook: &webhookActionInfo{
				TLSkipHostVerify: &falseValue,
			},
		}, nil, 200)
	defer svr.Close()
	defer stream.stop()
	// close the channel before stopping the server
	defer close(eventStream)

	p := newEvtProcessor("abc", stream)
	subInfo := &api.SubscriptionInfo{
		ID:          "abc",
		PayloadType: api.EventPayloadType_StringifiedJSON,
	}
	jsonstring := "{\"ID\":\"asset1072\",\"color\":\"yellow\",\"size\":10,\"owner\":\"Tom\",\"appraisedValue\":1300}"
	entry := &api.EventEntry{
		Payload: []byte(jsonstring),
	}
	err := p.processEventEntry(subInfo, entry)
	assert.NoError(err)
	decoded := entry.Payload.(map[string]interface{})
	assert.Equal("asset1072", decoded["ID"])

	subInfo.PayloadType = api.EventPayloadType_String
	entry.Payload = []byte(jsonstring)
	err = p.processEventEntry(subInfo, entry)
	assert.NoError(err)
	_, ok := entry.Payload.(string)
	assert.True(ok)
	assert.Equal(jsonstring, entry.Payload)

	subInfo.PayloadType = ""
	entry.Payload = []byte(jsonstring)
	err = p.processEventEntry(subInfo, entry)
	assert.NoError(err)
	_, ok = entry.Payload.([]byte)
	assert.True(ok)
	assert.Equal([]byte(jsonstring), entry.Payload)
}
