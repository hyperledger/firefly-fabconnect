// Copyright 22021 Kaleido
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

package kafka

import (
	"crypto/tls"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Shopify/sarama"
	"github.com/hyperledger/firefly-fabconnect/internal/conf"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	log "github.com/sirupsen/logrus"
)

// KafkaCommon is the base interface for bridges that interact with Kafka
type KafkaCommon interface {
	ValidateConf() error
	Start() error
	Conf() conf.KafkaConf
	Producer() KafkaProducer
}

// NewKafkaCommon constructs a new KafkaCommon instance
func NewKafkaCommon(kf KafkaFactory, conf conf.KafkaConf, kafkaGoRoutines KafkaGoRoutines) (k KafkaCommon) {
	k = &kafkaCommon{
		factory:         kf,
		kafkaGoRoutines: kafkaGoRoutines,
		conf:            conf,
	}
	return
}

// kafkaCommon provides a base command for establishing Kafka connectivity with a
// producer and a consumer-group
type kafkaCommon struct {
	conf            conf.KafkaConf
	factory         KafkaFactory
	client          KafkaClient
	signals         chan os.Signal
	consumer        KafkaConsumer
	consumerWG      sync.WaitGroup
	producer        KafkaProducer
	producerWG      sync.WaitGroup
	kafkaGoRoutines KafkaGoRoutines
	saramaLogger    saramaLogger
}

func (k *kafkaCommon) Conf() conf.KafkaConf {
	return k.conf
}

func (k *kafkaCommon) Producer() KafkaProducer {
	return k.producer
}

// ValidateConf performs common Cobra PreRunE logic for Kafka related commands
func (k *kafkaCommon) ValidateConf() error {
	return KafkaValidateConf(k.conf)
}

// KafkaValidateConf validates supplied configuration
func KafkaValidateConf(kconf conf.KafkaConf) (err error) {
	if kconf.TopicOut == "" {
		return errors.Errorf(errors.ConfigKafkaMissingOutputTopic)
	}
	if kconf.TopicIn == "" {
		return errors.Errorf(errors.ConfigKafkaMissingInputTopic)
	}
	if kconf.ConsumerGroup == "" {
		return errors.Errorf(errors.ConfigKafkaMissingConsumerGroup)
	}
	if !utils.AllOrNoneReqd(kconf.SASL.Username, kconf.SASL.Password) {
		err = errors.Errorf(errors.ConfigKafkaMissingBadSASL)
		return
	}
	return
}

type saramaLogger struct {
}

func (s saramaLogger) Print(v ...interface{}) {
	v = append([]interface{}{"[sarama] "}, v...)
	log.Debug(v...)
}

func (s saramaLogger) Printf(format string, v ...interface{}) {
	log.Debugf("[sarama] "+format, v...)
}

func (s saramaLogger) Println(v ...interface{}) {
	v = append([]interface{}{"[sarama] "}, v...)
	log.Debug(v...)
}

func (k *kafkaCommon) connect() (err error) {

	log.Debugf("Kafka Bootstrap brokers: %s", k.conf.Brokers)
	if len(k.conf.Brokers) == 0 || k.conf.Brokers[0] == "" {
		err = errors.Errorf(errors.ConfigKafkaMissingBrokers)
		return
	}

	sarama.Logger = k.saramaLogger
	clientConf := sarama.NewConfig()

	var tlsConfig *tls.Config
	if tlsConfig, err = utils.CreateTLSConfiguration(&k.conf.TLS); err != nil {
		return
	}

	if k.conf.SASL.Username != "" && k.conf.SASL.Password != "" {
		clientConf.Net.SASL.Enable = true
		clientConf.Net.SASL.User = k.conf.SASL.Username
		clientConf.Net.SASL.Password = k.conf.SASL.Password
	}

	clientConf.Producer.Return.Successes = true
	clientConf.Producer.Return.Errors = true
	clientConf.Producer.RequiredAcks = sarama.WaitForLocal
	clientConf.Producer.Flush.Frequency = time.Duration(k.conf.ProducerFlush.Frequency) * time.Millisecond
	clientConf.Producer.Flush.Messages = k.conf.ProducerFlush.Messages
	clientConf.Producer.Flush.Bytes = k.conf.ProducerFlush.Bytes
	clientConf.Metadata.Retry.Backoff = 2 * time.Second
	clientConf.Consumer.Return.Errors = true
	clientConf.Version = sarama.V2_0_0_0
	clientConf.Net.TLS.Enable = (tlsConfig != nil)
	clientConf.Net.TLS.Config = tlsConfig
	clientConf.ClientID = k.conf.ClientID
	if clientConf.ClientID == "" {
		clientConf.ClientID = utils.UUIDv4()
	}
	log.Debugf("Kafka ClientID: %s", clientConf.ClientID)

	if k.client, err = k.factory.NewClient(k, clientConf); err != nil {
		log.Errorf("Failed to create Kafka client: %s", err)
		return
	}
	var brokers []string
	for _, broker := range k.client.Brokers() {
		brokers = append(brokers, broker.Addr())
	}
	log.Infof("Kafka Connected: %s", brokers)

	return
}

func (k *kafkaCommon) createProducer() (err error) {
	log.Debugf("Kafka Producer Topic=%s", k.conf.TopicOut)
	if k.producer, err = k.client.NewProducer(k); err != nil {
		log.Errorf("Failed to create Kafka producer: %s", err)
		return
	}
	return
}

func (k *kafkaCommon) startProducer() (err error) {

	k.producerWG.Add(2)

	go k.kafkaGoRoutines.ProducerErrorLoop(k.consumer, k.producer, &k.producerWG)

	go k.kafkaGoRoutines.ProducerSuccessLoop(k.consumer, k.producer, &k.producerWG)

	log.Infof("Kafka Created producer")
	return
}

func (k *kafkaCommon) createConsumer() (err error) {
	log.Debugf("Kafka Consumer Topic=%s ConsumerGroup=%s", k.conf.TopicIn, k.conf.ConsumerGroup)
	if k.consumer, err = k.client.NewConsumer(k); err != nil {
		log.Errorf("Failed to create Kafka consumer: %s", err)
		return
	}
	return
}

func (k *kafkaCommon) startConsumer() (err error) {

	k.consumerWG.Add(2) // messages and errors
	go func() {
		for err := range k.consumer.Errors() {
			log.Error("Kafka consumer failed:", err)
		}
		k.consumerWG.Done()
	}()
	go k.kafkaGoRoutines.ConsumerMessagesLoop(k.consumer, k.producer, &k.consumerWG)

	log.Infof("Kafka Created consumer")
	return
}

// Start kicks off the bridge
func (k *kafkaCommon) Start() (err error) {

	if err = k.connect(); err != nil {
		return
	}
	if err = k.createConsumer(); err != nil {
		return
	}
	if err = k.createProducer(); err != nil {
		return
	}
	if err = k.startConsumer(); err != nil {
		return
	}
	if err = k.startProducer(); err != nil {
		return
	}

	log.Debugf("Kafka initialization complete")
	k.signals = make(chan os.Signal, 1)
	signal.Notify(k.signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	for range k.signals {
		k.producer.AsyncClose()
		k.consumer.Close()
		k.producerWG.Wait()
		k.consumerWG.Wait()

		log.Infof("Kafka Bridge complete")
		return
	}
	return
}
