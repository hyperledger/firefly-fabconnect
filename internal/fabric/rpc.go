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

package fabric

import (
	"context"
	"io/ioutil"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/retry"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	log "github.com/sirupsen/logrus"
)

type RPCClient interface {
	Init(ctx context.Context, channelId, chaincodeName, method string, args []string) (TxReceipt, error)
	Invoke(ctx context.Context, channelId, chaincodeName, method string, args []string) (TxReceipt, error)
	Close()
}

type rpcWrapper struct {
	sdk *fabsdk.FabricSDK
	// msp.IdentityIdentifier.ID <-> channel.Client
	channelMap map[string]*channel.Client
}

func RPCConnect(conf conf.RPCConf) (RPCClient, error) {
	dat, err := ioutil.ReadFile(conf.ConfigPath)
	if err != nil {
		log.Errorf("Failed to read common connection profile file: %s. %s", conf.ConfigPath, err)
		return nil, err
	}
	configProvider := config.FromRaw(dat, "yaml")
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		log.Errorf("Failed to initialize a new SDK instance. %s", err)
		return nil, err
	}

	log.Infof("New JSON/RPC connection established")
	return &rpcWrapper{sdk: sdk}, nil
}

func (w *rpcWrapper) Init(ctx context.Context, channelId, chaincodeName, method string, args []string) (TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)
	result, err := w.channelMap[channelId].Execute(
		channel.Request{ChaincodeID: chaincodeName, Fcn: method, Args: convert(args), IsInit: true},
		channel.WithRetry(retry.DefaultChannelOpts),
	)
	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return newReceipt(result), err
}

func (w *rpcWrapper) Invoke(ctx context.Context, channelId, chaincodeName, method string, args []string) (TxReceipt, error) {
	log.Tracef("RPC [%s:%s:%s] --> %+v", channelId, chaincodeName, method, args)
	result, err := w.channelMap[channelId].Execute(
		channel.Request{ChaincodeID: chaincodeName, Fcn: method, Args: convert(args)},
		channel.WithRetry(retry.DefaultChannelOpts),
	)
	log.Tracef("RPC [%s:%s:%s] <-- %+v", channelId, chaincodeName, method, result)
	return newReceipt(result), err
}

func (w *rpcWrapper) Close() {
	w.sdk.Close()
}

func convert(args []string) [][]byte {
	result := [][]byte{}
	for _, v := range args {
		result = append(result, []byte(v))
	}
	return result
}

func newReceipt(response channel.Response) TxReceipt {
	return TxReceipt{
		SignerMSP:     string(response.Proposal.Header),
		TransactionID: string(response.TransactionID),
		Status:        int(response.TxValidationCode),
	}
}

type mockRPCClient struct{}

func (m *mockRPCClient) Init(ctx context.Context, channelId, chaincodeName, method string, args []string) (TxReceipt, error) {
	return TxReceipt{}, nil
}
func (m *mockRPCClient) Invoke(ctx context.Context, channelId, chaincodeName, method string, args []string) (TxReceipt, error) {
	return TxReceipt{}, nil
}
func (m *mockRPCClient) Close() {}

func NewMockRPCClient() RPCClient {
	return &mockRPCClient{}
}
