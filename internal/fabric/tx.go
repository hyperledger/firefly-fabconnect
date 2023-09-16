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

package fabric

import (
	"context"
	"sync"
	"time"

	fabricClient "github.com/hyperledger/firefly-fabconnect/internal/fabric/client"
	messaging "github.com/hyperledger/firefly-fabconnect/internal/messages"

	"github.com/sirupsen/logrus"
)

// Txn wraps a Fabric transaction, along with the logic to send it over
// JSON/RPC to a node
type Tx struct {
	lock          sync.Mutex
	ChannelID     string
	ChaincodeName string
	IsInit        bool
	Function      string
	Args          []string
	TransientMap  map[string]string
	Hash          string
	Receipt       *fabricClient.TxReceipt
	Signer        string
}

func NewSendTx(msg *messaging.SendTransaction, _ string) *Tx {
	return &Tx{
		ChannelID:     msg.Headers.ChannelID,
		ChaincodeName: msg.Headers.ChaincodeName,
		IsInit:        msg.IsInit,
		Function:      msg.Function,
		Args:          msg.Args,
		TransientMap:  msg.TransientMap,
		Signer:        msg.Headers.Signer,
	}
}

// GetTXReceipt gets the receipt for the transaction
func (tx *Tx) GetTXReceipt(_ context.Context, _ fabricClient.RPCClient) (bool, error) {
	tx.lock.Lock()
	isMined := tx.Receipt.BlockNumber > 0
	tx.lock.Unlock()
	return isMined, nil
}

// Send sends an individual transaction
func (tx *Tx) Send(_ context.Context, rpc fabricClient.RPCClient) error {
	start := time.Now().UTC()

	var receipt *fabricClient.TxReceipt
	var err error
	receipt, err = rpc.Invoke(tx.ChannelID, tx.Signer, tx.ChaincodeName, tx.Function, tx.Args, tx.TransientMap, tx.IsInit)
	tx.lock.Lock()
	tx.Receipt = receipt
	tx.lock.Unlock()

	callTime := time.Now().UTC().Sub(start)
	if err != nil {
		logrus.Warnf("TX:%s Failed to send: %s [%.2fs]", tx.Hash, err, callTime.Seconds())
	} else {
		logrus.Infof("TX:%s Sent OK [%.2fs]", tx.Hash, callTime.Seconds())
	}
	return err
}
