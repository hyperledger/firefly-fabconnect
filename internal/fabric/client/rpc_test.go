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
	"net/http"
	"net/http/httptest"
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
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	mspApi "github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	mockfabricdep "github.com/hyperledger/firefly-fabconnect/mocks/fabric/dep"
	"github.com/julienschmidt/httprouter"
	"github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

	idcWrapper := wrapper.idClient.(*idClientWrapper)
	assert.Equal(3, len(idcWrapper.listeners))

	assert.NotEmpty(wrapper.channelClients["default-channel"]["user1"])
	idcWrapper.notifySignerUpdate("user1")
	assert.Empty(wrapper.channelClients["default-channel"]["user1"])
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
	client, err := wrapper.getGatewayClient("default-channel", "user1")
	assert.NoError(err)
	assert.NotNil(client)
	assert.Equal(1, len(wrapper.gwClients))
	assert.Equal(0, len(wrapper.gwChannelClients))

	gw := wrapper.gwClients["user1"]
	assert.NotNil(gw)
	assert.Equal(1, len(wrapper.gwGatewayClients))
	assert.Equal(1, len(wrapper.gwGatewayClients["user1"]))
	assert.Equal(client, wrapper.gwGatewayClients["user1"]["default-channel"])

	idcWrapper := wrapper.idClient.(*idClientWrapper)
	assert.Equal(3, len(idcWrapper.listeners))

	assert.NotEmpty(wrapper.gwGatewayClients["user1"])
	idcWrapper.notifySignerUpdate("user1")
	assert.Empty(wrapper.gwGatewayClients["user1"])
}

func TestGatewayClientSendTx(t *testing.T) {
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

	mockPrepareTx := func(w *gwRPCWrapper, signer, channelId, chaincodeName, method string, isInit bool) (*gateway.Transaction, <-chan *fab.TxStatusEvent, error) {
		notifier := make(chan *fab.TxStatusEvent)
		go func() {
			notifier <- &fab.TxStatusEvent{}
		}()
		return nil, notifier, nil
	}
	mockSubmitTx := func(tx *gateway.Transaction, transientMap map[string][]byte, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	wrapper.txPreparer = mockPrepareTx
	wrapper.txSubmitter = mockSubmitTx

	testmap := make(map[string]string)
	testmap["entry-1"] = "value-1"
	_, _, err = wrapper.sendTransaction("signer1", "channel-1", "chaincode-1", "method-1", []string{"args-1"}, testmap, false)
	assert.NoError(err)
}

func TestGatewayClientSendInitTx(t *testing.T) {
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

	mockPrepareTx := func(w *gwRPCWrapper, signer, channelId, chaincodeName, method string, isInit bool) (*gateway.Transaction, <-chan *fab.TxStatusEvent, error) {
		notifier := make(chan *fab.TxStatusEvent)
		go func() {
			notifier <- &fab.TxStatusEvent{}
		}()
		return nil, notifier, nil
	}
	mockSubmitTx := func(tx *gateway.Transaction, transientMap map[string][]byte, args ...string) ([]byte, error) {
		return []byte(""), nil
	}
	wrapper.txPreparer = mockPrepareTx
	wrapper.txSubmitter = mockSubmitTx

	testmap := make(map[string]string)
	testmap["entry-1"] = "value-1"
	_, _, err = wrapper.sendTransaction("signer1", "channel-1", "chaincode-1", "method-1", []string{"args-1"}, testmap, true)
	assert.NoError(err)
}

func TestChannelClientInstantiation(t *testing.T) {
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

	wrapper.channelCreator = createMockChannelClient
	client, err := wrapper.getChannelClient("default-channel", "user1")
	assert.NoError(err)
	assert.NotNil(client)
	assert.Equal(0, len(wrapper.gwClients))
	assert.Equal(0, len(wrapper.gwGatewayClients))
	assert.Equal(1, len(wrapper.gwChannelClients))
	assert.Equal(1, len(wrapper.gwChannelClients["user1"]))
	assert.Equal(client, wrapper.gwChannelClients["user1"]["default-channel"])

	idcWrapper := wrapper.idClient.(*idClientWrapper)
	assert.Equal(3, len(idcWrapper.listeners))

	assert.NotEmpty(wrapper.gwChannelClients["user1"])
	idcWrapper.notifySignerUpdate("user1")
	assert.Empty(wrapper.gwChannelClients["user1"])
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
	client1, err := wrapper.eventClientWrapper.getEventClient("default-channel", "user1", uint64(0), "chaincode-1")
	assert.NoError(err)
	assert.NotNil(client1)
	assert.Equal(1, len(wrapper.eventClientWrapper.eventClients))
	assert.Equal(1, len(wrapper.eventClientWrapper.eventClients["user1"]))
	assert.Equal(client1, wrapper.eventClientWrapper.eventClients["user1"]["default-channel-chaincode-1"])

	client2, err := wrapper.eventClientWrapper.getEventClient("default-channel", "user1", uint64(0), "chaincode-2")
	assert.NoError(err)
	assert.NotNil(client2)
	assert.Equal(1, len(wrapper.eventClientWrapper.eventClients))
	assert.Equal(2, len(wrapper.eventClientWrapper.eventClients["user1"]))
	assert.Equal(client2, wrapper.eventClientWrapper.eventClients["user1"]["default-channel-chaincode-2"])

	assert.NotEqual(fmt.Sprintf("%p", client1), fmt.Sprintf("%p", client2))

	idcWrapper := wrapper.eventClientWrapper.idClient.(*idClientWrapper)
	assert.Equal(3, len(idcWrapper.listeners))

	assert.NotEmpty(wrapper.eventClientWrapper.eventClients["user1"])
	idcWrapper.notifySignerUpdate("user1")
	assert.Empty(wrapper.eventClientWrapper.eventClients["user1"])
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

	idcWrapper := wrapper.ledgerClientWrapper.idClient.(*idClientWrapper)
	assert.Equal(3, len(idcWrapper.listeners))

	assert.NotEmpty(wrapper.ledgerClientWrapper.ledgerClients["user1"])
	idcWrapper.notifySignerUpdate("user1")
	assert.Empty(wrapper.ledgerClientWrapper.ledgerClients["user1"])
}

func TestIdentityRegister(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	idcWrapper := idclient.(*idClientWrapper)
	mockCAClient := mockfabricdep.CAClient{}
	mockCAClient.On("Register", mock.Anything).Return("mysecret", nil)
	idcWrapper.caClient = &mockCAClient

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/identities", strings.NewReader(`{"name":"user1","attributes":{"firstname":"John"}}`))
	r.Header.Set("Content-Type", "application/json")

	res, restErr := idclient.Register(w, r, httprouter.Params{})
	assert.Empty(restErr)
	assert.Equal("user1", res.Name)
	assert.Equal("mysecret", res.Secret)
}

func TestIdentityModify(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	idcWrapper := idclient.(*idClientWrapper)
	mockCAClient := mockfabricdep.CAClient{}
	mockCAClient.On("ModifyIdentity", mock.Anything).Return(&mspApi.IdentityResponse{}, nil)
	idcWrapper.caClient = &mockCAClient

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/identities/user1", strings.NewReader(`{"attributes":{"firstname":"John"}}`))
	r.Header.Set("Content-Type", "application/json")

	res, restErr := idclient.Modify(w, r, httprouter.Params{httprouter.Param{Key: "username", Value: "user1"}})
	assert.Empty(restErr)
	assert.Equal("user1", res.Name)
}

func TestIdentityEnroll(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	idcWrapper := idclient.(*idClientWrapper)
	mockCAClient := mockfabricdep.CAClient{}
	mockCAClient.On("Enroll", mock.Anything).Return(nil)
	idcWrapper.caClient = &mockCAClient

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/identities/user1/enroll", strings.NewReader(`{"name":"user1","secret":"mysecret","attributes":{"firstname":true}}`))
	r.Header.Set("Content-Type", "application/json")

	res, restErr := idclient.Enroll(w, r, httprouter.Params{httprouter.Param{Key: "username", Value: "user1"}})
	assert.Empty(restErr)
	assert.Equal("user1", res.Name)
	assert.Equal(true, res.Success)
}

func TestIdentityReenroll(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	idcWrapper := idclient.(*idClientWrapper)
	mockCAClient := mockfabricdep.CAClient{}
	mockCAClient.On("Reenroll", mock.Anything).Return(nil)
	idcWrapper.caClient = &mockCAClient

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/identities/user1/reenroll", strings.NewReader(`{"secret":"mysecret","attributes":{"firstname":true}}`))
	r.Header.Set("Content-Type", "application/json")

	res, restErr := idclient.Reenroll(w, r, httprouter.Params{httprouter.Param{Key: "username", Value: "user1"}})
	assert.Empty(restErr)
	assert.Equal("user1", res.Name)
	assert.Equal(true, res.Success)
}

func TestIdentityRevoke(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	idcWrapper := idclient.(*idClientWrapper)
	mockCAClient := mockfabricdep.CAClient{}
	mockCAClient.On("Revoke", mock.Anything).Return(&mspApi.RevocationResponse{
		CRL: []byte("some bytes"),
		RevokedCerts: []mspApi.RevokedCert{
			{
				Serial: "some-serial",
				AKI:    "some-aki",
			},
		},
	}, nil)
	idcWrapper.caClient = &mockCAClient

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/identities/user1/revoke", strings.NewReader(`{"reason":"getting old"}`))
	r.Header.Set("Content-Type", "application/json")

	res, restErr := idclient.Revoke(w, r, httprouter.Params{httprouter.Param{Key: "username", Value: "user1"}})
	assert.Empty(restErr)
	assert.Equal([]byte("some bytes"), res.CRL)
	assert.Equal(1, len(res.RevokedCerts))
	assert.Equal("some-serial", res.RevokedCerts[0]["serial"])
	assert.Equal("some-aki", res.RevokedCerts[0]["aki"])
}

func TestIdentityList(t *testing.T) {
	assert := assert.New(t)

	config := conf.RPCConf{
		ConfigPath: tmpCCPFile,
	}
	rpc, idclient, err := RPCConnect(config, 5)
	assert.NoError(err)
	assert.NotNil(rpc)
	assert.NotNil(idclient)

	idcWrapper := idclient.(*idClientWrapper)
	mockCAClient := mockfabricdep.CAClient{}
	mockCAClient.On("GetAllIdentities", mock.Anything).Return([]*mspApi.IdentityResponse{
		{
			ID:     "user1",
			CAName: "myca",
		},
	}, nil)
	idcWrapper.caClient = &mockCAClient

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/identities", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/json")

	res, restErr := idclient.List(w, r, httprouter.Params{})
	assert.Empty(restErr)
	assert.Equal(1, len(res))
	assert.Equal("user1", res[0].Name)
	assert.Equal("myca", res[0].CAName)
}
