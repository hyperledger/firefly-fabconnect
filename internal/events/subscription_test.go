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
	"fmt"
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/fabric/test"
	"github.com/stretchr/testify/assert"
)

func TestCreateWebhookSub(t *testing.T) {
	assert := assert.New(t)

	rpc := test.MockRPCClient("")
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
	assert.Equal("", s.info.Filter.EventFilter)
	assert.Equal("FromBlock=,Chaincode=,Filter=", s1.info.Summary)
	assert.Equal("", s.info.Filter.BlockType)
}

func TestRestoreSubscriptionMissingID(t *testing.T) {
	assert := assert.New(t)
	m := &mockSubMgr{err: fmt.Errorf("nope")}
	testInfo := testSubInfo("")
	testInfo.ID = ""
	_, err := restoreSubscription(m.stream, nil, testInfo)
	assert.NoError(err)
}
