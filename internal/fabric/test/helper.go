// Copyright 2019 Kaleido

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	eventmocks "github.com/hyperledger/fabric-sdk-go/pkg/fab/events/service/mocks"
	"github.com/hyperledger/firefly-fabconnect/internal/fabric/utils"
	mockfabric "github.com/hyperledger/firefly-fabconnect/mocks/fabric/client"
	"github.com/stretchr/testify/mock"
)

func MockRPCClient(fromBlock string, withReset ...bool) *mockfabric.RPCClient {
	rpc := &mockfabric.RPCClient{}
	blockEventChan := make(chan *fab.BlockEvent)
	ccEventChan := make(chan *fab.CCEvent)
	var roBlockEventChan <-chan *fab.BlockEvent = blockEventChan
	var roCCEventChan <-chan *fab.CCEvent = ccEventChan
	res := &fab.BlockchainInfoResponse{
		BCI: &common.BlockchainInfo{
			Height: 10,
		},
	}
	rawBlock := &utils.RawBlock{
		Header: &common.BlockHeader{
			Number: uint64(20),
		},
	}
	tx1 := &utils.Transaction{
		Timestamp: 1000000,
		TxId:      "3144a3ad43dcc11374832bbb71561320de81fd80d69cc8e26a9ea7d3240a5e84",
	}
	block := &utils.Block{
		Number:       uint64(20),
		Transactions: []*utils.Transaction{tx1},
	}
	txResult := make(map[string]interface{})
	chaincodeResult := []byte(`{"AppraisedValue":123000,"Color":"red","ID":"asset01","Owner":"Tom","Size":10}`)
	txResult["transaction"] = tx1
	rpc.On("SubscribeEvent", mock.Anything, mock.Anything).Return(nil, roBlockEventChan, roCCEventChan, nil)
	rpc.On("Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(chaincodeResult, nil)
	rpc.On("QueryChainInfo", mock.Anything, mock.Anything).Return(res, nil)
	rpc.On("QueryBlock", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(rawBlock, block, nil)
	rpc.On("QueryBlockByTxId", mock.Anything, mock.Anything, mock.Anything).Return(rawBlock, block, nil)
	rpc.On("QueryTransaction", mock.Anything, mock.Anything, mock.Anything).Return(txResult, nil)
	rpc.On("Unregister", mock.Anything).Return()

	go func() {
		if fromBlock == "0" {
			blockEventChan <- &fab.BlockEvent{
				Block: constructBlock(1),
			}
		}
		blockEventChan <- &fab.BlockEvent{
			Block: constructBlock(11),
		}
		ccEventChan <- &fab.CCEvent{
			BlockNumber: uint64(10),
			TxID:        "3144a3ad43dcc11374832bbb71561320de81fd80d69cc8e26a9ea7d3240a5e84",
			ChaincodeID: "asset_transfer",
		}
		if len(withReset) > 0 {
			blockEventChan <- &fab.BlockEvent{
				Block: constructBlock(11),
			}
		}
	}()

	return rpc
}

func constructBlock(number uint64) *common.Block {
	mockTx := eventmocks.NewTransactionWithCCEvent("testTxID", peer.TxValidationCode_VALID, "testChaincodeID", "testCCEventName", []byte("testPayload"))
	mockBlock := eventmocks.NewBlock("testChannelID", mockTx)
	mockBlock.Header.Number = number
	return mockBlock
}
