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

package auth

import (
	"context"
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/auth/authtest"
	"github.com/stretchr/testify/assert"
)

func TestSystemContext(t *testing.T) {
	assert := assert.New(t)

	assert.True(IsSystemContext(NewSystemAuthContext()))
	assert.False(IsSystemContext(context.Background()))
	assert.False(IsSystemContext(context.WithValue(context.Background(), ContextKeySystemAuth, "false")))
	assert.False(IsSystemContext(context.WithValue(context.Background(), ContextKeySystemAuth, false)))

}

func TestAccessToken(t *testing.T) {
	assert := assert.New(t)

	ctx, err := WithAuthContext(context.Background(), "testat")
	assert.NoError(err)
	assert.Equal("", GetAccessToken(ctx))
	assert.Equal(nil, GetAuthContext(ctx))

	RegisterSecurityModule(&authtest.TestSecurityModule{})

	ctx, err = WithAuthContext(context.Background(), "testat")
	assert.NoError(err)
	assert.Equal("verified", GetAuthContext(ctx))
	assert.Equal("testat", GetAccessToken(ctx))

	assert.Equal(nil, GetAuthContext(context.Background()))
	assert.Equal("", GetAccessToken(context.Background()))

	_, err = WithAuthContext(context.Background(), "badone")
	assert.EqualError(err, "badness")

	RegisterSecurityModule(nil)
}

func TestAuthRPC(t *testing.T) {
	assert := assert.New(t)

	assert.NoError(RPC(context.Background(), "anything"))

	RegisterSecurityModule(&authtest.TestSecurityModule{})

	assert.EqualError(RPC(context.Background(), "anything"), "No auth context")

	assert.NoError(RPC(NewSystemAuthContext(), "anything"))

	ctx, _ := WithAuthContext(context.Background(), "testat")
	assert.NoError(RPC(ctx, "testrpc"))
	assert.EqualError(RPC(ctx, "anything"), "badness")

	RegisterSecurityModule(nil)

}

func TestAuthRPCSubscribe(t *testing.T) {
	assert := assert.New(t)

	assert.NoError(RPCSubscribe(context.Background(), "anything", nil))

	RegisterSecurityModule(&authtest.TestSecurityModule{})

	assert.EqualError(RPCSubscribe(context.Background(), "anything", nil), "No auth context")

	assert.NoError(RPCSubscribe(NewSystemAuthContext(), "anything", nil))

	ctx, _ := WithAuthContext(context.Background(), "testat")
	assert.NoError(RPCSubscribe(ctx, "testns", nil))
	assert.EqualError(RPCSubscribe(ctx, "anything", nil), "badness")

	RegisterSecurityModule(nil)

}

func TestAuthEventStreams(t *testing.T) {
	assert := assert.New(t)

	assert.NoError(EventStreams(context.Background()))

	RegisterSecurityModule(&authtest.TestSecurityModule{})

	assert.EqualError(EventStreams(context.Background()), "No auth context")

	assert.NoError(EventStreams(NewSystemAuthContext()))

	ctx, _ := WithAuthContext(context.Background(), "testat")
	assert.NoError(EventStreams(ctx))

	RegisterSecurityModule(nil)

}

func TestAuthListAsyncReplies(t *testing.T) {
	assert := assert.New(t)

	assert.NoError(ListAsyncReplies(context.Background()))

	RegisterSecurityModule(&authtest.TestSecurityModule{})

	assert.EqualError(ListAsyncReplies(context.Background()), "No auth context")

	assert.NoError(ListAsyncReplies(NewSystemAuthContext()))

	ctx, _ := WithAuthContext(context.Background(), "testat")
	assert.NoError(ListAsyncReplies(ctx))

	RegisterSecurityModule(nil)

}

func TestAuthReadAsyncReplyByUUID(t *testing.T) {
	assert := assert.New(t)

	assert.NoError(ReadAsyncReplyByUUID(context.Background()))

	RegisterSecurityModule(&authtest.TestSecurityModule{})

	assert.EqualError(ReadAsyncReplyByUUID(context.Background()), "No auth context")

	assert.NoError(ReadAsyncReplyByUUID(NewSystemAuthContext()))

	ctx, _ := WithAuthContext(context.Background(), "testat")
	assert.NoError(ReadAsyncReplyByUUID(ctx))

	RegisterSecurityModule(nil)

}
