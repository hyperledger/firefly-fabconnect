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

package async

import (
	"context"
	"testing"

	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	mockreceipt "github.com/hyperledger/firefly-fabconnect/mocks/rest/receipt"
	mocktx "github.com/hyperledger/firefly-fabconnect/mocks/tx"
	"github.com/stretchr/testify/assert"
)

// type mockHandler struct{}

// func (*mockHandler) validateHandlerConf() error {
// 	return nil
// }

// func (*mockHandler) sendWebhookMsg(ctx context.Context, key, msgID string, msg map[string]interface{}, ack bool) (msgAck string, statusCode int, err error) {
// 	return "", 200, nil
// }

// func (*mockHandler) run() error {
// 	return nil
// }

// func (*mockHandler) isInitialized() bool {
// 	return true
// }

func TestDispatchMsgAsyncInvalidMsg(t *testing.T) {
	assert := assert.New(t)

	testConfig := &conf.RESTGatewayConf{}
	asyncD := NewAsyncDispatcher(testConfig, &mocktx.TxProcessor{}, &mockreceipt.ReceiptStore{})

	fakeMsg := messages.SendTransaction{}
	_, err := asyncD.DispatchMsgAsync(context.Background(), &fakeMsg, true)
	assert.EqualError(err, "Invalid message type: \"\"")
}
