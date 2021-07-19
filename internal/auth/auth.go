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

package auth

import (
	"context"
)

type ContextKey int

const (
	ContextKeySystemAuth ContextKey = iota
	ContextKeyAuthContext
	ContextKeyAccessToken
)

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

// AuthRPC authorize an RPC call
func AuthRPC(ctx context.Context, method string, args ...interface{}) error {
	return nil
}

// AuthRPCSubscribe authorize a subscribe RPC call
func AuthRPCSubscribe(ctx context.Context, namespace string, channel interface{}, args ...interface{}) error {
	return nil
}

// AuthEventStreams authorize the whole of event streams
func AuthEventStreams(ctx context.Context) error {
	return nil
}

// AuthListAsyncReplies authorize the listing or searching of all replies
func AuthListAsyncReplies(ctx context.Context) error {
	return nil
}

// AuthReadAsyncReplyByUUID authorize the query of an invidual reply by UUID
func AuthReadAsyncReplyByUUID(ctx context.Context) error {
	return nil
}
