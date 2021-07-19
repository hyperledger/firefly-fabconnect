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

package conf

import (
	"os"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// RESTGatewayConf defines the YAML config structure for a webhooks bridge instance
type RESTGatewayConf struct {
	MaxInFlight     int             `json:"maxInFlight"`
	MaxTXWaitTime   int             `json:"maxTXWaitTime"`
	SendConcurrency int             `json:"sendConcurrency"`
	Kafka           KafkaConf       `json:"kafka"`
	Receipts        ReceiptsDBConf  `json:"receipts"`
	Events          EventstreamConf `json:"events"`
	HTTP            HTTP            `json:"http"`
	RPC             RPCConf         `json:"rpc"`
}

// KafkaConf - Common configuration for Kafka
type KafkaConf struct {
	Brokers       []string `json:"brokers"`
	ClientID      string   `json:"clientID"`
	ConsumerGroup string   `json:"consumerGroup"`
	TopicIn       string   `json:"topicIn"`
	TopicOut      string   `json:"topicOut"`
	ProducerFlush struct {
		Frequency int `json:"frequency"`
		Messages  int `json:"messages"`
		Bytes     int `json:"bytes"`
	} `json:"producerFlush"`
	SASL struct {
		Username string
		Password string
	} `json:"sasl"`
	TLS TLSConfig `json:"tls"`
}

type ReceiptsDBConf struct {
	MaxDocs             int                 `json:"maxDocs"`
	QueryLimit          int                 `json:"queryLimit"`
	RetryInitialDelayMS int                 `json:"retryInitialDelay"`
	RetryTimeoutMS      int                 `json:"retryTimeout"`
	MongoDB             MongoDBReceiptsConf `json:"mongodb"`
	LevelDB             LevelDBReceiptsConf `json:"leveldb"`
}

// MongoDBReceiptStoreConf is the configuration for a MongoDB receipt store
type MongoDBReceiptsConf struct {
	URL              string `json:"url"`
	Database         string `json:"database"`
	Collection       string `json:"collection"`
	ConnectTimeoutMS int    `json:"connectTimeout"`
}

type LevelDBReceiptsConf struct {
	Path string `json:"path"`
}

type EventstreamConf struct {
	PollingIntervalSec uint64              `json:"pollingInterval"`
	LevelDB            LevelDBReceiptsConf `json:"leveldb"`
}

type RPCConf struct {
	ConfigPath string `json:"config-path"`
}

type HTTP struct {
	LocalAddr string    `json:"localAddr"`
	Port      int       `json:"port"`
	TLS       TLSConfig `json:"tls"`
}

// TLSConfig is the common TLS config
type TLSConfig struct {
	ClientCertsFile    string `json:"clientCertsFile"`
	ClientKeyFile      string `json:"clientKeyFile"`
	CACertsFile        string `json:"caCertsFile"`
	Enabled            bool   `json:"enabled"`
	InsecureSkipVerify bool   `json:"insecureSkipVerify"`
}

// CobraInitRPC sets the standard command-line parameters for RPC
func CobraInit(cmd *cobra.Command, conf RESTGatewayConf) {
	cmd.Flags().IntVarP(&conf.MaxInFlight, "maxinflight", "m", DefInt("WEBHOOKS_MAX_INFLIGHT", 0), "Maximum messages to hold in-flight")
	cmd.Flags().IntVarP(&conf.MaxTXWaitTime, "tx-timeout", "x", DefInt("ETH_TX_TIMEOUT", 0), "Maximum wait time for an individual transaction (seconds)")
	cmd.Flags().StringVarP(&conf.HTTP.LocalAddr, "listen-addr", "L", os.Getenv("WEBHOOKS_LISTEN_ADDR"), "Local address to listen on")
	cmd.Flags().IntVarP(&conf.HTTP.Port, "listen-port", "l", DefInt("WEBHOOKS_LISTEN_PORT", 8080), "Port to listen on")

	cmd.Flags().IntVarP(&conf.Receipts.MaxDocs, "receipt-maxdocs", "X", DefInt("RECEIPT_MAXDOCS", 0), "Receipt store capped size (new collections only)")
	cmd.Flags().IntVarP(&conf.Receipts.QueryLimit, "receipt-query-limit", "Q", DefInt("RECEIPT_QUERYLIM", 0), "Maximum docs to return on a rest call (cap on limit)")
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.URL, "mongodb-url", "M", os.Getenv("MONGODB_URL"), "MongoDB URL for a receipt store")
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.Database, "mongodb-database", "D", os.Getenv("MONGODB_DATABASE"), "MongoDB receipt store database")
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.Collection, "mongodb-receipt-collection", "R", os.Getenv("MONGODB_COLLECTION"), "MongoDB receipt store collection")
	cmd.Flags().StringVarP(&conf.Receipts.LevelDB.Path, "leveldb-path", "B", os.Getenv("LEVELDB_PATH"), "Path to LevelDB data directory")

	cmd.Flags().StringVarP(&conf.Events.LevelDB.Path, "events-db", "E", "", "Level DB location for subscription management")
	cmd.Flags().Uint64VarP(&conf.Events.PollingIntervalSec, "events-polling-int", "j", 10, "Event polling interval (ms)")

	defBrokerList := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(defBrokerList) == 1 && defBrokerList[0] == "" {
		defBrokerList = []string{}
	}
	defTLSenabled, _ := strconv.ParseBool(os.Getenv("KAFKA_TLS_ENABLED"))
	defTLSinsecure, _ := strconv.ParseBool(os.Getenv("KAFKA_TLS_INSECURE"))
	cmd.Flags().StringArrayVarP(&conf.Kafka.Brokers, "brokers", "b", defBrokerList, "Comma-separated list of bootstrap brokers")
	cmd.Flags().StringVarP(&conf.Kafka.ClientID, "clientid", "i", os.Getenv("KAFKA_CLIENT_ID"), "Client ID (or generated UUID)")
	cmd.Flags().StringVarP(&conf.Kafka.ConsumerGroup, "consumer-group", "g", os.Getenv("KAFKA_CONSUMER_GROUP"), "Client ID (or generated UUID)")
	cmd.Flags().StringVarP(&conf.Kafka.TopicIn, "topic-in", "t", os.Getenv("KAFKA_TOPIC_IN"), "Topic to listen to")
	cmd.Flags().StringVarP(&conf.Kafka.TopicOut, "topic-out", "T", os.Getenv("KAFKA_TOPIC_OUT"), "Topic to send events to")
	cmd.Flags().StringVarP(&conf.Kafka.TLS.ClientCertsFile, "tls-clientcerts", "c", os.Getenv("KAFKA_TLS_CLIENT_CERT"), "A client certificate file, for mutual TLS auth")
	cmd.Flags().StringVarP(&conf.Kafka.TLS.ClientKeyFile, "tls-clientkey", "k", os.Getenv("KAFKA_TLS_CLIENT_KEY"), "A client private key file, for mutual TLS auth")
	cmd.Flags().StringVarP(&conf.Kafka.TLS.CACertsFile, "tls-cacerts", "C", os.Getenv("KAFKA_TLS_CA_CERTS"), "CA certificates file (or host CAs will be used)")
	cmd.Flags().BoolVarP(&conf.Kafka.TLS.Enabled, "tls-enabled", "e", defTLSenabled, "Encrypt network connection with TLS (SSL)")
	cmd.Flags().BoolVarP(&conf.Kafka.TLS.InsecureSkipVerify, "tls-insecure", "z", defTLSinsecure, "Disable verification of TLS certificate chain")
	cmd.Flags().StringVarP(&conf.Kafka.SASL.Username, "sasl-username", "u", os.Getenv("KAFKA_SASL_USERNAME"), "Username for SASL authentication")
	cmd.Flags().StringVarP(&conf.Kafka.SASL.Password, "sasl-password", "p", os.Getenv("KAFKA_SASL_PASSWORD"), "Password for SASL authentication")

	cmd.Flags().StringVarP(&conf.RPC.ConfigPath, "rpc-config", "r", os.Getenv("RPC_CONFIG"), "Path to the common connection profile YAML for the target Fabric node")
}

// DefInt defaults an integer to a value in an Env var, and if not the default integer provided
func DefInt(envVarName string, defValue int) int {
	defStr := os.Getenv(envVarName)
	if defStr == "" {
		return defValue
	}
	parsedInt, err := strconv.ParseInt(defStr, 10, 32)
	if err != nil {
		log.Errorf("Invalid string in env var %s", envVarName)
		return defValue
	}
	return int(parsedInt)
}
