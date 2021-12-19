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

package fabric

import (
	"context"
	"time"

	"github.com/hyperledger/firefly-fabconnect/internal/fabric/client"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"

	log "github.com/sirupsen/logrus"
)

// Query wraps a Fabric transaction, along with the logic to send it over
// JSON/RPC to a node
type Query struct {
	ChannelID     string
	ChaincodeName string
	Function      string
	Args          []string
	Signer        string
}

func NewQuery(msg *messages.QueryChaincode, signer string) *Query {
	return &Query{
		ChannelID:     msg.Headers.ChannelID,
		ChaincodeName: msg.Headers.ChaincodeName,
		Function:      msg.Function,
		Args:          msg.Args,
		Signer:        msg.Headers.Signer,
	}
}

// Send sends an individual query
func (tx *Query) Send(ctx context.Context, rpc client.RPCClient) ([]byte, error) {
	start := time.Now().UTC()

	var err error
	result, err := rpc.Query(tx.ChannelID, tx.Signer, tx.ChaincodeName, tx.Function, tx.Args)
	callTime := time.Now().UTC().Sub(start)
	if err != nil {
		log.Warnf("Query [chaincode=%s, func=%s] failed to send: %s [%.2fs]", tx.ChaincodeName, tx.Function, err, callTime.Seconds())
	} else {
		log.Infof("Query [chaincode=%s, func=%s] [%.2fs]", tx.ChaincodeName, tx.Function, callTime.Seconds())
	}
	return result, err
}
