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

package async

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/hyperledger/firefly-fabconnect/internal/auth"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/kafka"
	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/receipt"
	log "github.com/sirupsen/logrus"
)

// kafkaHandler provides the HTTP -> Kafka bridge functionality
type kafkaHandler struct {
	kafka       kafka.KafkaCommon
	receipts    receipt.ReceiptStore
	sendCond    *sync.Cond
	pendingMsgs map[string]bool
	successMsgs map[string]*sarama.ProducerMessage
	failedMsgs  map[string]error
	finished    bool
}

// newWebhooksKafka constructor
func newKafkaHandler(kconf conf.KafkaConf, receipts receipt.ReceiptStore) *kafkaHandler {
	w := &kafkaHandler{
		receipts:    receipts,
		sendCond:    sync.NewCond(&sync.Mutex{}),
		pendingMsgs: make(map[string]bool),
		successMsgs: make(map[string]*sarama.ProducerMessage),
		failedMsgs:  make(map[string]error),
	}
	kf := &kafka.SaramaKafkaFactory{}
	w.kafka = kafka.NewKafkaCommon(kf, kconf, w)
	return w
}

func (w *kafkaHandler) setMsgPending(msgID string) {
	w.sendCond.L.Lock()
	w.pendingMsgs[msgID] = true
	w.sendCond.L.Unlock()
}

func (w *kafkaHandler) waitForSend(msgID string) (msg *sarama.ProducerMessage, err error) {
	w.sendCond.L.Lock()
	for msg == nil && err == nil {
		var found bool
		if err, found = w.failedMsgs[msgID]; found {
			delete(w.failedMsgs, msgID)
		} else if msg, found = w.successMsgs[msgID]; found {
			delete(w.successMsgs, msgID)
		} else {
			w.sendCond.Wait()
		}
	}
	w.sendCond.L.Unlock()
	return
}

// ConsumerMessagesLoop - consume replies
func (w *kafkaHandler) ConsumerMessagesLoop(consumer kafka.KafkaConsumer, producer kafka.KafkaProducer, wg *sync.WaitGroup) {
	for msg := range consumer.Messages() {
		w.receipts.ProcessReceipt(msg.Value)

		// Regardless of outcome, we ack
		consumer.MarkOffset(msg, "")
	}
	wg.Done()
}

// ProducerErrorLoop - consume errors
func (w *kafkaHandler) ProducerErrorLoop(consumer kafka.KafkaConsumer, producer kafka.KafkaProducer, wg *sync.WaitGroup) {
	log.Debugf("Kafka handler listening for errors sending to Kafka")
	for err := range producer.Errors() {
		log.Errorf("Error sending message: %s", err)
		if err.Msg == nil || err.Msg.Metadata == nil {
			// This should not be possible
			panic(errors.Errorf(errors.WebhooksKafkaUnexpectedErrFmt, err))
		}
		msgID := err.Msg.Metadata.(string)
		w.sendCond.L.Lock()
		if _, found := w.pendingMsgs[msgID]; found {
			delete(w.pendingMsgs, msgID)
			w.failedMsgs[msgID] = err
			w.sendCond.Broadcast()
		}
		w.sendCond.L.Unlock()
	}
	wg.Done()
}

// ProducerSuccessLoop - consume successes
func (w *kafkaHandler) ProducerSuccessLoop(consumer kafka.KafkaConsumer, producer kafka.KafkaProducer, wg *sync.WaitGroup) {
	log.Debugf("Kafka handler listening for successful sends to Kafka")
	for msg := range producer.Successes() {
		log.Infof("Kafka handler sent message ok: %s", msg.Metadata)
		if msg.Metadata == nil {
			// This should not be possible
			panic(errors.Errorf(errors.WebhooksKafkaDeliveryReportNoMeta, msg))
		}
		msgID := msg.Metadata.(string)
		w.sendCond.L.Lock()
		if _, found := w.pendingMsgs[msgID]; found {
			delete(w.pendingMsgs, msgID)
			w.successMsgs[msgID] = msg
			w.sendCond.Broadcast()
		}
		w.sendCond.L.Unlock()
	}
	wg.Done()
}

func (w *kafkaHandler) dispatchMsg(ctx context.Context, key, msgID string, msg *messages.SendTransaction, ack bool) (string, int, error) {

	// Reseialize back to JSON with the headers
	payloadToForward, err := json.Marshal(&msg)
	if err != nil {
		return "", 500, errors.Errorf(errors.WebhooksKafkaMsgtoJSON, err)
	}
	if ack {
		w.setMsgPending(msgID)
	}

	log.Debugf("Message payload: %s", payloadToForward)
	sentMsg := &sarama.ProducerMessage{
		Topic:    w.kafka.Conf().TopicOut,
		Key:      sarama.StringEncoder(key),
		Value:    sarama.ByteEncoder(payloadToForward),
		Metadata: msgID,
	}
	accessToken := auth.GetAccessToken(ctx)
	if accessToken != "" {
		sentMsg.Headers = []sarama.RecordHeader{
			{
				Key:   []byte(messages.RecordHeaderAccessToken),
				Value: []byte(accessToken),
			},
		}
	}
	w.kafka.Producer().Input() <- sentMsg

	msgAck := ""
	if ack {
		successMsg, err := w.waitForSend(msgID)
		if err != nil {
			return "", 502, errors.Errorf(errors.WebhooksKafkaErr, err)
		}
		msgAck = fmt.Sprintf("%s:%d:%d", successMsg.Topic, successMsg.Partition, successMsg.Offset)
	}
	return msgAck, 200, nil
}

func (w *kafkaHandler) validateHandlerConf() error {
	return w.kafka.ValidateConf()
}

func (w *kafkaHandler) run() error {
	err := w.kafka.Start()
	w.finished = true
	return err
}

func (w *kafkaHandler) isInitialized() bool {
	// We mark ourselves as ready once the kafka bridge has constructed its
	// producer, so it can accept messages.
	return (w.finished || w.kafka.Producer() != nil)
}
