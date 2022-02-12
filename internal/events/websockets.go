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

package events

import (
	"fmt"

	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/events/api"
	log "github.com/sirupsen/logrus"
)

type webSocketAction struct {
	es   *eventStream
	spec *webSocketActionInfo
}

func newWebSocketAction(es *eventStream, spec *webSocketActionInfo) (*webSocketAction, error) {
	if es.wsChannels == nil {
		return nil, errors.Errorf(errors.EventStreamsWebSocketNotConfigured)
	}
	if spec == nil {
		spec = &webSocketActionInfo{
			Topic: "",
		}
	}
	return &webSocketAction{
		es:   es,
		spec: spec,
	}, nil
}

func validateWebsocketConfig(spec *webSocketActionInfo) error {
	if spec.Topic == "" {
		return fmt.Errorf("Missing required parameter 'websocket.topic'")
	}
	sd := spec.DistributionMode
	if sd != "" && sd != DistributionModeBroadcast && sd != DistributionModeWLD {
		return errors.Errorf(errors.EventStreamsInvalidDistributionMode, sd)
	}
	return nil
}

// attemptBatch attempts to deliver a batch over socket IO
func (w *webSocketAction) attemptBatch(batchNumber, attempt uint64, events []*api.EventEntry) error {
	var err error

	// Get a blocking channel to send and receive on our chosen namespace
	sender, broadcaster, receiver, closing := w.es.wsChannels.GetChannels(w.spec.Topic)

	log.Debugf("attempting batch %d with %d events", batchNumber, len(events))

	var channel chan<- interface{}
	switch w.spec.DistributionMode {
	case DistributionModeBroadcast:
		channel = broadcaster
	default:
		channel = sender
	}

	// Sent the batch of events
	select {
	case channel <- events:
		break
	case <-w.es.updateInterrupt:
		return errors.Errorf(errors.EventStreamsWebSocketInterruptedSend)
	case <-closing:
		return errors.Errorf(errors.EventStreamsWebSocketInterruptedSend)
	}

	// If we ever add more distribution modes, we may want to change this logic from a simple if statement
	if w.spec.DistributionMode != DistributionModeBroadcast {
		// Wait for the next ack or exception
		select {
		case err = <-receiver:
			break
		case <-w.es.updateInterrupt:
			return errors.Errorf(errors.EventStreamsWebSocketInterruptedReceive)
		case <-closing:
			return errors.Errorf(errors.EventStreamsWebSocketInterruptedReceive)
		}
		// Pass back any exception from the client
	}
	return err
}
