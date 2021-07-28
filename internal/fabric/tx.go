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
	"time"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/messages"
	pb "github.com/hyperledger/fabric-protos-go/peer"

	log "github.com/sirupsen/logrus"
)

// Txn wraps a Fabric transaction, along with the logic to send it over
// JSON/RPC to a node
type Tx struct {
	ChannelID     string
	ChaincodeName string
	Function      string
	Args          []string
	Hash          string
	Receipt       *TxReceipt
	Signer        string
}

type TxReceipt struct {
	BlockNumber   uint64              `json:"blockNumber"`
	SignerMSP     string              `json:"signerMSP"`
	Signer        string              `json:"signer"`
	ChaincodeSpec ChaincodeSpec       `json:"chaincode"`
	TransactionID string              `json:"transactionID"`
	Status        pb.TxValidationCode `json:"status"`
	SourcePeer    string              `json:"peer"`
}

type ChaincodeSpec struct {
	Type    int    `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// NewSendTxn builds a new ethereum transaction from the supplied
// SendTranasction message
func NewSendTx(msg *messages.SendTransaction, signer string) (tx *Tx, err error) {
	if tx, err = buildTX(signer, msg.Headers.ChannelID, msg.ChaincodeName, msg.Function, msg.Args); err != nil {
		return
	}
	return
}

func buildTX(signer, channelID, chaincodeName, function string, args []string) (tx *Tx, err error) {
	tx = &Tx{
		ChannelID:     channelID,
		ChaincodeName: chaincodeName,
		Function:      function,
		Args:          args,
		Signer:        signer,
	}

	return
}

// GetTXReceipt gets the receipt for the transaction
func (tx *Tx) GetTXReceipt(ctx context.Context, rpc RPCClient) (bool, error) {
	isMined := tx.Receipt.BlockNumber > 0
	return isMined, nil
}

// Send sends an individual transaction, choosing external or internal signing
func (tx *Tx) Send(ctx context.Context, rpc RPCClient) (err error) {
	start := time.Now().UTC()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	tx.Receipt, err = rpc.Invoke(ctx, tx.ChannelID, tx.Signer, tx.ChaincodeName, tx.Function, tx.Args)

	callTime := time.Now().UTC().Sub(start)
	if err != nil {
		log.Warnf("TX:%s Failed to send: %s [%.2fs]", tx.Hash, err, callTime.Seconds())
	} else {
		log.Infof("TX:%s Sent OK [%.2fs]", tx.Hash, callTime.Seconds())
	}
	return err
}

func (r *TxReceipt) IsSuccess() bool {
	return r.Status == pb.TxValidationCode_VALID
}
