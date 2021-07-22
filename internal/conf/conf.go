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

	"github.com/spf13/cobra"
)

// mapstructure instead of json is used for tagging the properties here
// in order to work with spf13/viper unmarshaling

// RESTGatewayConf defines the YAML config structure for a webhooks bridge instance
type RESTGatewayConf struct {
	MaxInFlight     int             `mapstructure:"maxInFlight"`
	MaxTXWaitTime   int             `mapstructure:"maxTXWaitTime"`
	SendConcurrency int             `mapstructure:"sendConcurrency"`
	Kafka           KafkaConf       `mapstructure:"kafka"`
	Receipts        ReceiptsDBConf  `mapstructure:"receipts"`
	Events          EventstreamConf `mapstructure:"events"`
	HTTP            HTTPConf        `mapstructure:"http"`
	RPC             RPCConf         `mapstructure:"rpc"`
}

// KafkaConf - Common configuration for Kafka
type KafkaConf struct {
	Brokers       []string `mapstructure:"brokers"`
	ClientID      string   `mapstructure:"clientID"`
	ConsumerGroup string   `mapstructure:"consumerGroup"`
	TopicIn       string   `mapstructure:"topicIn"`
	TopicOut      string   `mapstructure:"topicOut"`
	ProducerFlush struct {
		Frequency int `mapstructure:"frequency"`
		Messages  int `mapstructure:"messages"`
		Bytes     int `mapstructure:"bytes"`
	} `mapstructure:"producerFlush"`
	SASL struct {
		Username string
		Password string
	} `mapstructure:"sasl"`
	TLS TLSConfig `mapstructure:"tls"`
}

type ReceiptsDBConf struct {
	MaxDocs             int                 `mapstructure:"maxDocs"`
	QueryLimit          int                 `mapstructure:"queryLimit"`
	RetryInitialDelayMS int                 `mapstructure:"retryInitialDelay"`
	RetryTimeoutMS      int                 `mapstructure:"retryTimeout"`
	MongoDB             MongoDBReceiptsConf `mapstructure:"mongodb"`
	LevelDB             LevelDBReceiptsConf `mapstructure:"leveldb"`
}

// MongoDBReceiptStoreConf is the configuration for a MongoDB receipt store
type MongoDBReceiptsConf struct {
	URL              string `mapstructure:"url"`
	Database         string `mapstructure:"database"`
	Collection       string `mapstructure:"collection"`
	ConnectTimeoutMS int    `mapstructure:"connectTimeout"`
}

type LevelDBReceiptsConf struct {
	Path string `mapstructure:"path"`
}

type EventstreamConf struct {
	PollingIntervalSec uint64              `mapstructure:"pollingInterval"`
	LevelDB            LevelDBReceiptsConf `mapstructure:"leveldb"`
}

type RPCConf struct {
	ConfigPath string `mapstructure:"configPath"`
}

type HTTPConf struct {
	LocalAddr string    `mapstructure:"localAddr"`
	Port      int       `mapstructure:"port"`
	TLS       TLSConfig `mapstructure:"tls"`
}

// TLSConfig is the common TLS config
type TLSConfig struct {
	ClientCertsFile    string `mapstructure:"clientCertsFile"`
	ClientKeyFile      string `mapstructure:"clientKeyFile"`
	CACertsFile        string `mapstructure:"caCertsFile"`
	Enabled            bool   `mapstructure:"enabled"`
	InsecureSkipVerify bool   `mapstructure:"insecureSkipVerify"`
}

// CobraInitRPC sets the standard command-line parameters for RPC
func CobraInit(cmd *cobra.Command, conf *RESTGatewayConf) {
	cmd.Flags().IntVarP(&conf.MaxInFlight, "maxinflight", "m", 0, "Maximum messages to hold in-flight")
	cmd.Flags().IntVarP(&conf.MaxTXWaitTime, "tx-timeout", "x", 0, "Maximum wait time for an individual transaction (seconds)")
	cmd.Flags().StringVarP(&conf.HTTP.LocalAddr, "listen-addr", "L", "", "Local address to listen on")
	cmd.Flags().IntVarP(&conf.HTTP.Port, "listen-port", "l", 8080, "Port to listen on")

	cmd.Flags().IntVarP(&conf.Receipts.MaxDocs, "receipt-maxdocs", "X", 0, "Receipt store capped size (new collections only)")
	cmd.Flags().IntVarP(&conf.Receipts.QueryLimit, "receipt-query-limit", "Q", 0, "Maximum docs to return on a rest call (cap on limit)")
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.URL, "mongodb-url", "M", "", "MongoDB URL for a receipt store")
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.Database, "mongodb-database", "D", "", "MongoDB receipt store database")
	cmd.Flags().StringVarP(&conf.Receipts.MongoDB.Collection, "mongodb-receipt-collection", "R", "", "MongoDB receipt store collection")
	cmd.Flags().StringVarP(&conf.Receipts.LevelDB.Path, "leveldb-path", "B", "", "Path to LevelDB data directory")

	cmd.Flags().StringVarP(&conf.Events.LevelDB.Path, "events-db", "E", "", "Level DB location for subscription management")
	cmd.Flags().Uint64VarP(&conf.Events.PollingIntervalSec, "events-polling-int", "j", 10, "Event polling interval (ms)")

	defBrokerList := strings.Split(os.Getenv("KAFKA_BROKERS"), ",")
	if len(defBrokerList) == 1 && defBrokerList[0] == "" {
		defBrokerList = []string{}
	}
	defTLSenabled, _ := strconv.ParseBool(os.Getenv("KAFKA_TLS_ENABLED"))
	defTLSinsecure, _ := strconv.ParseBool(os.Getenv("KAFKA_TLS_INSECURE"))
	cmd.Flags().StringArrayVarP(&conf.Kafka.Brokers, "brokers", "b", defBrokerList, "Comma-separated list of bootstrap brokers")
	cmd.Flags().StringVarP(&conf.Kafka.ClientID, "clientid", "i", "", "Client ID (or generated UUID)")
	cmd.Flags().StringVarP(&conf.Kafka.ConsumerGroup, "consumer-group", "g", "", "Client ID (or generated UUID)")
	cmd.Flags().StringVarP(&conf.Kafka.TopicIn, "topic-in", "t", "", "Topic to listen to")
	cmd.Flags().StringVarP(&conf.Kafka.TopicOut, "topic-out", "T", "", "Topic to send events to")
	cmd.Flags().StringVarP(&conf.Kafka.TLS.ClientCertsFile, "tls-clientcerts", "c", "", "A client certificate file, for mutual TLS auth")
	cmd.Flags().StringVarP(&conf.Kafka.TLS.ClientKeyFile, "tls-clientkey", "k", "", "A client private key file, for mutual TLS auth")
	cmd.Flags().StringVarP(&conf.Kafka.TLS.CACertsFile, "tls-cacerts", "C", "", "CA certificates file (or host CAs will be used)")
	cmd.Flags().BoolVarP(&conf.Kafka.TLS.Enabled, "tls-enabled", "e", defTLSenabled, "Encrypt network connection with TLS (SSL)")
	cmd.Flags().BoolVarP(&conf.Kafka.TLS.InsecureSkipVerify, "tls-insecure", "z", defTLSinsecure, "Disable verification of TLS certificate chain")
	cmd.Flags().StringVarP(&conf.Kafka.SASL.Username, "sasl-username", "u", "", "Username for SASL authentication")
	cmd.Flags().StringVarP(&conf.Kafka.SASL.Password, "sasl-password", "p", "", "Password for SASL authentication")

	cmd.Flags().StringVarP(&conf.RPC.ConfigPath, "rpc-config", "r", "", "Path to the common connection profile YAML for the target Fabric node")
}
