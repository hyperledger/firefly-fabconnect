package client

import (
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
)

func getOrgFromConfig(config core.ConfigProvider) (string, error) {
	configBackend, err := config()
	if err != nil {
		return "", err
	}
	if len(configBackend) != 1 {
		return "", errors.Errorf("Invalid config file")
	}

	cfg := configBackend[0]
	value, ok := cfg.Lookup("client.organization")
	if !ok {
		return "", errors.Errorf("No client organization defined in the config")
	}

	return value.(string), nil
}

func getFirstPeerEndpointFromConfig(config core.ConfigProvider) (string, error) {
	org, err := getOrgFromConfig(config)
	if err != nil {
		return "", err
	}
	configBackend, _ := config()
	cfg := configBackend[0]
	value, ok := cfg.Lookup(fmt.Sprintf("organizations.%s.peers", org))
	if !ok {
		return "", errors.Errorf("No peers list found in the organization %s", org)
	}
	peers := value.([]interface{})
	if len(peers) < 1 {
		return "", errors.Errorf("Peers list for organization %s is empty", org)
	}
	return peers[0].(string), nil
}

// defined to allow mocking in tests
type channelCreator func(context.ChannelProvider) (*channel.Client, error)

func createChannelClient(channelProvider context.ChannelProvider) (*channel.Client, error) {
	return channel.New(channelProvider)
}
