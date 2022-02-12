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

package events

import (
	"encoding/json"
	"sync"

	"github.com/hyperledger/firefly-fabconnect/internal/events/api"
	log "github.com/sirupsen/logrus"
)

type eventData struct {
	event         *api.EventEntry
	batchComplete func(*api.EventEntry)
}

type evtProcessor struct {
	subID    string
	stream   *eventStream
	blockHWM uint64
	hwmSync  sync.Mutex
}

func newEvtProcessor(subID string, stream *eventStream) *evtProcessor {
	return &evtProcessor{
		subID:  subID,
		stream: stream,
	}
}

func (ep *evtProcessor) batchComplete(newestEvent *api.EventEntry) {
	ep.hwmSync.Lock()
	newHWM := newestEvent.BlockNumber + 1
	if newHWM > ep.blockHWM {
		ep.blockHWM = newHWM
	}
	ep.hwmSync.Unlock()
	log.Debugf("%s: High-Water-Mark: %d", ep.subID, ep.blockHWM)
}

func (ep *evtProcessor) getBlockHWM() uint64 {
	ep.hwmSync.Lock()
	v := ep.blockHWM
	ep.hwmSync.Unlock()
	return v
}

func (ep *evtProcessor) initBlockHWM(intVal uint64) {
	ep.hwmSync.Lock()
	ep.blockHWM = intVal
	ep.hwmSync.Unlock()
}

func (ep *evtProcessor) processEventEntry(subInfo *api.SubscriptionInfo, entry *api.EventEntry) (err error) {
	entry.SubID = subInfo.ID
	payloadType := subInfo.PayloadType
	if payloadType == "" {
		payloadType = api.EventPayloadType_Bytes
	}

	// if the payload comes from the block decoder, it's already decoded into a map
	// first check if it's still a byte array
	payloadBytes, ok := entry.Payload.([]byte)
	if ok {
		switch payloadType {
		case api.EventPayloadType_String:
			entry.Payload = string(payloadBytes)
		case api.EventPayloadType_StringifiedJSON, api.EventPayloadType_JSON:
			structuredMap := make(map[string]interface{})
			err := json.Unmarshal(payloadBytes, &structuredMap)
			if err != nil {
				log.Errorf("Failed to unmarshal event payload for [sub:%s,name:%s,block=%d]", entry.SubID, entry.EventName, entry.BlockNumber)
			} else {
				entry.Payload = structuredMap
			}
		}
	}

	result := eventData{
		event:         entry,
		batchComplete: ep.batchComplete,
	}

	// Ok, now we have the full event in a friendly map output. Pass it down to the stream
	log.Infof("%s: Dispatching event. BlockNumber=%d TxId=%s", subInfo.ID, result.event.BlockNumber, result.event.TransactionId)
	ep.stream.eventHandler(&result)
	return nil
}
