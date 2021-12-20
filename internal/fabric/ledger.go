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

type LedgerQuery struct {
	ChannelID string
	Signer    string
	TxId      string
}

func NewLedgerQuery(msg *messages.GetTxById, signer string) *LedgerQuery {
	return &LedgerQuery{
		ChannelID: msg.Headers.ChannelID,
		Signer:    msg.Headers.Signer,
		TxId:      msg.TxId,
	}
}

// Send sends an individual query
func (q *LedgerQuery) Send(ctx context.Context, rpc client.RPCClient) (map[string]interface{}, error) {
	start := time.Now().UTC()

	var err error
	result, err := rpc.QueryTransaction(q.ChannelID, q.Signer, q.TxId)
	callTime := time.Now().UTC().Sub(start)
	if err != nil {
		log.Warnf("Query transaction %s failed to send: %s [%.2fs]", q.TxId, err, callTime.Seconds())
	} else {
		log.Infof("Query transaction %s [%.2fs]", q.TxId, callTime.Seconds())
	}
	return result, err
}
