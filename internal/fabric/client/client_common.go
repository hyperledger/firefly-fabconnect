package client

import (
	"fmt"

	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/fabsdk"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	eventsapi "github.com/hyperledger/firefly-fabconnect/internal/events/api"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/utils"
	log "github.com/sirupsen/logrus"
)

type commonRPCWrapper struct {
	txTimeout           int
	configProvider      core.ConfigProvider
	sdk                 *fabsdk.FabricSDK
	idClient            IdentityClient
	ledgerClientWrapper *ledgerClientWrapper
	eventClientWrapper  *eventClientWrapper
	channelCreator      channelCreator
}

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

func newReceipt(responsePayload []byte, status *fab.TxStatusEvent, signerID *msp.IdentityIdentifier) *TxReceipt {
	return &TxReceipt{
		SignerMSP:     signerID.MSPID,
		Signer:        signerID.ID,
		TransactionID: status.TxID,
		Status:        status.TxValidationCode,
		BlockNumber:   status.BlockNumber,
		SourcePeer:    status.SourceURL,
	}
}

func convertStringArray(args []string) [][]byte {
	result := [][]byte{}
	for _, v := range args {
		result = append(result, []byte(v))
	}
	return result
}

func convertStringMap(_map map[string]string) map[string][]byte {
	result := make(map[string][]byte, len(_map))
	for k, v := range _map {
		result[k] = []byte(v)
	}
	return result
}

func (w *commonRPCWrapper) QueryChainInfo(channelId, signer string) (*fab.BlockchainInfoResponse, error) {
	log.Tracef("RPC [%s] --> QueryChainInfo", channelId)

	result, err := w.ledgerClientWrapper.queryChainInfo(channelId, signer)
	if err != nil {
		log.Errorf("Failed to query chain info on channel %s. %s", channelId, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

func (w *commonRPCWrapper) QueryBlock(channelId string, signer string, blockNumber uint64, blockhash []byte) (*utils.RawBlock, *utils.Block, error) {
	log.Tracef("RPC [%s] --> QueryBlock %v", channelId, blockNumber)

	rawblock, block, err := w.ledgerClientWrapper.queryBlock(channelId, signer, blockNumber, blockhash)
	if err != nil {
		log.Errorf("Failed to query block %v on channel %s. %s", blockNumber, channelId, err)
		return nil, nil, err
	}

	log.Tracef("RPC [%s] <-- success", channelId)
	return rawblock, block, nil
}

func (w *commonRPCWrapper) QueryBlockByTxId(channelId string, signer string, txId string) (*utils.RawBlock, *utils.Block, error) {
	log.Tracef("RPC [%s] --> QueryBlockByTxId %s", channelId, txId)

	rawblock, block, err := w.ledgerClientWrapper.queryBlockByTxId(channelId, signer, txId)
	if err != nil {
		log.Errorf("Failed to query block by transaction Id %s on channel %s. %s", txId, channelId, err)
		return nil, nil, err
	}

	log.Tracef("RPC [%s] <-- success", channelId)
	return rawblock, block, nil
}

func (w *commonRPCWrapper) QueryTransaction(channelId, signer, txId string) (map[string]interface{}, error) {
	log.Tracef("RPC [%s] --> QueryTransaction %s", channelId, txId)

	result, err := w.ledgerClientWrapper.queryTransaction(channelId, signer, txId)
	if err != nil {
		log.Errorf("Failed to query transaction on channel %s. %s", channelId, err)
		return nil, err
	}

	log.Tracef("RPC [%s] <-- %+v", channelId, result)
	return result, nil
}

// The returned registration must be closed when done
func (w *commonRPCWrapper) SubscribeEvent(subInfo *eventsapi.SubscriptionInfo, since uint64) (*RegistrationWrapper, <-chan *fab.BlockEvent, <-chan *fab.CCEvent, error) {
	reg, blockEventCh, ccEventCh, err := w.eventClientWrapper.subscribeEvent(subInfo, since)
	if err != nil {
		log.Errorf("Failed to subscribe to event [%s:%s:%s]. %s", subInfo.Stream, subInfo.ChannelId, subInfo.Filter.ChaincodeId, err)
		return nil, nil, nil, err
	}
	return reg, blockEventCh, ccEventCh, nil
}

func (w *commonRPCWrapper) Unregister(regWrapper *RegistrationWrapper) {
	regWrapper.eventClient.Unregister(regWrapper.registration)
}
