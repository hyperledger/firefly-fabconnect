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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/event"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/ledger"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
)

var tmpdir string
var tmpConfigFile string
var tmpCCPFile string
var tmpShortConfigFile string
var tmpShortCCPFile string

func TestMain(m *testing.M) {
	var code int
	err := setup()
	if err != nil {
		fmt.Printf("Failed to set up test: %s", err)
		os.Exit(1)
	}
	code = m.Run()
	teardown()
	os.Exit(code)
}

func newTempdir() string {
	dir, _ := ioutil.TempDir("", "rpc_test")
	fmt.Printf("tmpdir/create: %s\n", dir)
	return dir
}

func cleanup(dir string) {
	fmt.Printf("tmpdir/cleanup: %s\n", dir)
	os.RemoveAll(dir)
}

func setup() error {
	tmpdir = newTempdir()
	_, sourcedir, _, _ := runtime.Caller(0)
	configFile, err := setupConfigFile("config.json", sourcedir, tmpdir)
	if err != nil {
		return err
	}
	tmpConfigFile = configFile

	configFile, err = setupConfigFile("ccp.yml", sourcedir, tmpdir)
	if err != nil {
		return err
	}
	tmpCCPFile = configFile

	configFile, err = setupConfigFile("config-short.json", sourcedir, tmpdir)
	if err != nil {
		return err
	}
	tmpShortConfigFile = configFile

	configFile, err = setupConfigFile("ccp-short.yml", sourcedir, tmpdir)
	if err != nil {
		return err
	}
	tmpShortCCPFile = configFile

	err = copy.Copy(path.Join(sourcedir, "../../../../test/fixture/nodeMSPs"), path.Join(tmpdir, "nodeMSPs"))
	if err != nil {
		return err
	}
	err = copy.Copy(path.Join(sourcedir, "../../../../test/fixture/org1"), path.Join(tmpdir, "org1"))
	if err != nil {
		return err
	}

	return nil
}

func teardown() {
	cleanup(tmpdir)
}

func setupConfigFile(filename, sourcedir, targetdir string) (string, error) {
	configFile := path.Join(sourcedir, "../../../../test/fixture", filename)
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		return "", err
	}
	contentStr := strings.ReplaceAll(string(content), "{{ROOT_DIR}}", tmpdir)
	configFile = path.Join(targetdir, filename)
	err = os.WriteFile(configFile, []byte(contentStr), 0644)
	if err != nil {
		return "", err
	}
	return configFile, nil
}

func createMockChannelClient(channelProvider context.ChannelProvider) (*channel.Client, error) {
	return &channel.Client{}, nil
}

func createMockGateway(configProvider core.ConfigProvider, signer string, txTimeout int) (*gateway.Gateway, error) {
	return &gateway.Gateway{}, nil
}

func createMockNetwork(gw *gateway.Gateway, channelId string) (*gateway.Network, error) {
	return &gateway.Network{}, nil
}

func createMockEventClient(channelProvider context.ChannelProvider, opts ...event.ClientOption) (*event.Client, error) {
	return &event.Client{}, nil
}

func createMockLedgerClient(channelProvider context.ChannelProvider, opts ...ledger.ClientOption) (*ledger.Client, error) {
	return &ledger.Client{}, nil
}

func TestCCPClientInstantiation(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	wrapper, ok := rpc.(*ccpRPCWrapper)
	assert.True(ok)

	wrapper.channelCreator = createMockChannelClient
	client, err := wrapper.getChannelClient("default-channel", "user1")
	assert.NoError(err)
	assert.NotNil(client)
	assert.Equal(1, len(wrapper.channelClients))
	assert.Equal(1, len(wrapper.channelClients["default-channel"]))
}

func TestGatewayClientInstantiation(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		UseGatewayClient: true,
		ConfigPath:       tmpShortCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	wrapper, ok := rpc.(*gwRPCWrapper)
	assert.True(ok)

	wrapper.gatewayCreator = createMockGateway
	wrapper.networkCreator = createMockNetwork
	client, err := wrapper.getChannelClient("default-channel", "user1")
	assert.NoError(err)
	assert.NotNil(client)
	assert.Equal(1, len(wrapper.gwClients))

	gw := wrapper.gwClients["user1"]
	assert.NotNil(gw)
	assert.Equal(1, len(wrapper.gwChannelClients))
	assert.Equal(1, len(wrapper.gwChannelClients["user1"]))
	assert.Equal(client, wrapper.gwChannelClients["user1"]["default-channel"])
}

func TestEventClientInstantiation(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	wrapper, ok := rpc.(*ccpRPCWrapper)
	assert.True(ok)

	wrapper.eventClientWrapper.eventClientCreator = createMockEventClient
	client, err := wrapper.eventClientWrapper.getEventClient("default-channel", "user1", uint64(0))
	assert.NoError(err)
	assert.NotNil(client)
	assert.Equal(1, len(wrapper.eventClientWrapper.eventClients))
	assert.Equal(1, len(wrapper.eventClientWrapper.eventClients["user1"]))
	assert.Equal(client, wrapper.eventClientWrapper.eventClients["user1"]["default-channel"])
}

func TestLedgerClientInstantiation(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	wrapper, ok := rpc.(*ccpRPCWrapper)
	assert.True(ok)

	wrapper.ledgerClientWrapper.ledgerClientCreator = createMockLedgerClient
	client, err := wrapper.ledgerClientWrapper.getLedgerClient("default-channel", "user1")
	assert.NoError(err)
	assert.NotNil(client)
	assert.Equal(1, len(wrapper.ledgerClientWrapper.ledgerClients))
	assert.Equal(1, len(wrapper.ledgerClientWrapper.ledgerClients["user1"]))
	assert.Equal(client, wrapper.ledgerClientWrapper.ledgerClients["user1"]["default-channel"])
}
