// Copyright © 2023 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"context"

	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/pkg/plugins"
)

type ContextKey int

const (
	ContextKeySystemAuth ContextKey = iota
	ContextKeyAuthContext
	ContextKeyAccessToken
)

var securityModule plugins.SecurityModule

// RegisterSecurityModule is the plug point to register a security module
func RegisterSecurityModule(sm plugins.SecurityModule) {
	securityModule = sm
}

// NewSystemAuthContext creates a system background context
func NewSystemAuthContext() context.Context {
	return context.WithValue(context.Background(), ContextKeySystemAuth, true)
}

// IsSystemContext checks if a context was created as a system context
func IsSystemContext(ctx context.Context) bool {
	b, ok := ctx.Value(ContextKeySystemAuth).(bool)
	return ok && b
}

// WithAuthContext adds an access token to a base context
func WithAuthContext(ctx context.Context, token string) (context.Context, error) {
	if securityModule != nil {
		ctxValue, err := securityModule.VerifyToken(token)
		if err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, ContextKeyAccessToken, token)
		ctx = context.WithValue(ctx, ContextKeyAuthContext, ctxValue)
		return ctx, nil
	}
	return ctx, nil
}

// GetAuthContext extracts a previously stored auth context from the context
func GetAuthContext(ctx context.Context) interface{} {
	return ctx.Value(ContextKeyAuthContext)
}

// GetAccessToken extracts a previously stored access token
func GetAccessToken(ctx context.Context) string {
	v, ok := ctx.Value(ContextKeyAccessToken).(string)
	if ok {
		return v
	}
	return ""
}

// RPC authorize an RPC call
func RPC(ctx context.Context, method string, args ...interface{}) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthRPC(authCtx, method, args...)
	}
	return nil
}

// RPCSubscribe authorize a subscribe RPC call
func RPCSubscribe(ctx context.Context, namespace string, channel interface{}, args ...interface{}) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthRPCSubscribe(authCtx, namespace, channel, args...)
	}
	return nil
}

// EventStreams authorize the whole of event streams
func EventStreams(ctx context.Context) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthEventStreams(authCtx)
	}
	return nil
}

// ListAsyncReplies authorize the listing or searching of all replies
func ListAsyncReplies(ctx context.Context) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthListAsyncReplies(authCtx)
	}
	return nil
}

// ReadAsyncReplyByUUID authorize the query of an invidual reply by UUID
func ReadAsyncReplyByUUID(ctx context.Context) error {
	if securityModule != nil && !IsSystemContext(ctx) {
		authCtx := GetAuthContext(ctx)
		if authCtx == nil {
			return errors.Errorf(errors.SecurityModuleNoAuthContext)
		}
		return securityModule.AuthReadAsyncReplyByUUID(authCtx)
	}
	return nil
}
