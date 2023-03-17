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

package client

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/identity"
	log "github.com/sirupsen/logrus"
)

// Instantiate an RPC client to interact with a Fabric network. based on the client configuration
// on gateway usage, it creates different types of client under the cover:
// - "useGatewayClient: true": returned RPCClient uses the client-side Gateway
// - "useGatewayClient: false": returned RPCClient uses a static network map described by the Connection Profile
// - "useGatewayServer: true": for Fabric 2.4 node only, the returned RPCClient utilizes the server-side gateway service
func RPCConnect(c conf.RPCConf, txTimeout int) (RPCClient, identity.Client, error) {
	configProvider := config.FromFile(c.ConfigPath)
	userStore, err := newUserstore(configProvider)
	if err != nil {
		return nil, nil, errors.Errorf("User credentials store creation failed. %s", err)
	}
	identityClient, err := newIdentityClient(configProvider, userStore)
	if err != nil {
		return nil, nil, err
	}
	sdk, err := fabsdk.New(configProvider)
	if err != nil {
		return nil, nil, errors.Errorf("Failed to initialize a new SDK instance. %s", err)
	}
	ledgerClient := newLedgerClient(configProvider, sdk, identityClient)
	eventClient := newEventClient(configProvider, sdk, identityClient)
	var rpcClient RPCClient
	if !c.UseGatewayClient && !c.UseGatewayServer {
		rpcClient, err = newRPCClientFromCCP(configProvider, txTimeout, userStore, identityClient, ledgerClient, eventClient)
		if err != nil {
			return nil, nil, err
		}
		log.Info("Using static connection profile mode of the RPC client")
	} else if c.UseGatewayClient {
		rpcClient, err = newRPCClientWithClientSideGateway(configProvider, txTimeout, identityClient, ledgerClient, eventClient)
		if err != nil {
			return nil, nil, err
		}
		log.Info("Using client-side gateway mode of the RPC client")
	}
	return rpcClient, identityClient, nil
}
