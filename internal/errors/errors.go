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

package errors

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// ErrorID enumerates all errors in ethconnect.
type ErrorID string
type Error string

func (e Error) Error() string {
	return string(e)
}

// Errorf creates an error (not yet translated, but an extensible interface for that using simple sprintf formatting rather than named i18n inserts)
func Errorf(msg ErrorID, inserts ...interface{}) error {
	var err error = Error(fmt.Sprintf(string(msg), inserts...))
	return errors.WithStack(err)
}

const (
	// ConfigFileReadFailed failed to read the server config file
	ConfigFileReadFailed = "Failed to read %s: %s"
	// ConfigNoYAML missing configuration file on server start
	ConfigFileMissing = "No configuration filename specified"
	// ConfigYAMLParseFile failed to parse YAML during server startup
	ConfigYAMLParseFile = "Unable to parse %s as YAML: %s"
	// ConfigYAMLPostParseFile failed to process YAML as JSON after parsing
	ConfigYAMLPostParseFile = "Failed to process YAML config from %s: %s"
	// ConfigRESTGatewayRequiredHTTPPort for rest server listening port missing
	ConfigRESTGatewayRequiredHTTPPort = "Must provide REST Gateway http listening port"
	// ConfigRESTGatewayRequiredRPCPath for rest server's Fabric client config file missing
	ConfigRESTGatewayRequiredRPCPath = "Must provide REST Gateway client configuration path"
	// ConfigRESTGatewayRequiredReceiptStore need to enable params for REST Gatewya
	ConfigRESTGatewayRequiredReceiptStore = "MongoDB URL, Database and Collection name must be specified to enable the receipt store"
	// ConfigTLSCertOrKey incomplete TLS config
	ConfigTLSCertOrKey = "Client private key and certificate must both be provided for mutual auth"

	// SecurityModulePluginLoad failed to load .so
	SecurityModulePluginLoad = "Failed to load plugin: %s"
	// SecurityModulePluginSymbol missing symbol in plugin
	SecurityModulePluginSymbol = "Failed to load 'SecurityModule' symbol from '%s': %s"
	// SecurityModuleNoAuthContext missing auth context in context object at point security module is invoked
	SecurityModuleNoAuthContext = "No auth context"

	// RequestHandlerInvalidMsgHeaders missing headers section in the JSON/YAML posted
	RequestHandlerInvalidMsgHeaders = "Invalid message - missing 'headers' (or not an object)"
	// RequestHandlerInvalidMsgTypeMissing need to specify a msg type in the header
	RequestHandlerInvalidMsgTypeMissing = "Invalid message - missing 'headers.type' (or not a string)"
	// RequestHandlerInvalidMsgFromMissing need to specify a msg type in the header
	RequestHandlerInvalidMsgFromMissing = "Invalid message - missing 'from' (or not a string)"
	// RequestHandlerInvalidMsgType need to specify a valid msg type in the header
	RequestHandlerInvalidMsgType = "Invalid message type: %s"

	// RequestHandlerDirectTooManyInflight when we're not using a buffered store (Kafka) we have to reject
	RequestHandlerDirectTooManyInflight = "Too many in-flight transactions"
	// RequestHandlerDirectBadHeaders problem processing for in-memory operation
	RequestHandlerDirectBadHeaders = "Failed to process headers in message"

	// TransactionSendMsgTypeUnknown we got a JSON message into the core processor (from Kafka, Webhooks etc.) that we don't understand
	TransactionSendMsgTypeUnknown = "Unknown message type '%s'"

	// TransactionSendReceiptCheckError we continually had bad RCs back from the node while trying to check for the receipt up to the timeout
	TransactionSendReceiptCheckError = "Error obtaining transaction receipt (%d retries): %s"
	// TransactionSendReceiptCheckTimeout we didn't have a problem asking the node for a receipt, but the transaction wasn't mined at the end of the timeout
	TransactionSendReceiptCheckTimeout = "Timed out waiting for transaction receipt"

	// RPCCallReturnedError specified RPC call returned error
	RPCCallReturnedError = "%s returned: %s"
	// RPCConnectFailed error connecting to back-end server over JSON/RPC
	RPCConnectFailed = "JSON/RPC connection to %s failed: %s"

	// RESTGatewayMissingFromAddress did not supply a signing address for the transaction
	RESTGatewayMissingSigner = "Please specify a valid signer ID in the '%[1]s-signer' query string parameter or x-%[2]s-signer HTTP header"
	// RESTGatewaySyncMsgTypeMismatch sync-invoke code paths in REST API Gateway should be maintained such that this cannot happen
	RESTGatewaySyncMsgTypeMismatch = "Unexpected condition (message types do not match when processing)"
	// RESTGatewaySyncWrapErrorWithTXDetail wraps a low level error with transaction hash context on sync APIs before returning
	RESTGatewaySyncWrapErrorWithTXDetail = "TX %s: %s"

	// ConfigKafkaMissingOutputTopic response topic missing
	ConfigKafkaMissingOutputTopic = "No output topic specified for bridge to send events to"
	// ConfigKafkaMissingInputTopic request topic missing
	ConfigKafkaMissingInputTopic = "No input topic specified for bridge to listen to"
	// ConfigKafkaMissingConsumerGroup consumer group missing
	ConfigKafkaMissingConsumerGroup = "No consumer group specified"
	// ConfigKafkaMissingBadSASL problem with SASL config
	ConfigKafkaMissingBadSASL = "Username and Password must both be provided for SASL"
	// ConfigKafkaMissingBrokers missing/empty brokers
	ConfigKafkaMissingBrokers = "No Kafka brokers configured"
	// WebhooksKafkaUnexpectedErrFmt problem processing an error that came back from Kafka, so do a deep dump
	WebhooksKafkaUnexpectedErrFmt = "Error did not contain message and metadata: %+v"
	// WebhooksKafkaDeliveryReportNoMeta delivery reports should contain the metadata we set when we sent
	WebhooksKafkaDeliveryReportNoMeta = "Sent message did not contain metadata: %+v"
	// WebhooksKafkaYAMLtoJSON re-serialization of webhook message into JSON failed
	WebhooksKafkaYAMLtoJSON = "Unable to reserialize YAML payload as JSON: %s"
	// WebhooksKafkaErr wrapper on detailed error from Kafka itself
	WebhooksKafkaErr = "Failed to deliver message to Kafka: %s"

	// HelperYAMLorJSONPayloadTooLarge input message too large
	HelperYAMLorJSONPayloadTooLarge = "Message exceeds maximum allowable size"
	// HelperYAMLorJSONPayloadReadFailed failed to read input
	HelperYAMLorJSONPayloadReadFailed = "Unable to read input data: %s"
	// HelperYAMLorJSONPayloadParseFailed input message got error parsing
	HelperYAMLorJSONPayloadParseFailed = "Unable to parse as YAML or JSON: %s"

	// ReceiptStoreDisabled not configured
	ReceiptStoreDisabled = "Receipt store not enabled"
	// ReceiptStoreSerializeResponse problem sending a receipt stored back over the REST API
	ReceiptStoreSerializeResponse = "Error serializing response"
	// ReceiptStoreInvalidRequestID bad ID query
	ReceiptStoreInvalidRequestID = "Invalid 'id' query parameter"
	// ReceiptStoreInvalidRequestMaxLimit bad limit over max
	ReceiptStoreInvalidRequestMaxLimit = "Maximum limit is %d"
	// ReceiptStoreInvalidRequestBadLimit bad limit
	ReceiptStoreInvalidRequestBadLimit = "Invalid 'limit' query parameter"
	// ReceiptStoreInvalidRequestBadSkip bad skip
	ReceiptStoreInvalidRequestBadSkip = "Invalid 'skip' query parameter"
	// ReceiptStoreInvalidRequestBadSince bad since
	ReceiptStoreInvalidRequestBadSince = "since cannot be parsed as RFC3339 or millisecond timestamp"
	// ReceiptStoreFailedQuery wrapper over detailed error
	ReceiptStoreFailedQuery = "Error querying replies: %s"
	// ReceiptStoreFailedQuerySingle wrapper over detailed error
	ReceiptStoreFailedQuerySingle = "Error querying reply: %s"
	// ReceiptStoreFailedNotFound receipt isn't in the store
	ReceiptStoreFailedNotFound = "Receipt not available"
	// ReceiptStoreMongoDBConnect couldn't connect to MongoDB
	ReceiptStoreMongoDBConnect = "Unable to connect to MongoDB: %s"
	// ReceiptStoreMongoDBIndex couldn't create MongoDB index
	ReceiptStoreMongoDBIndex = "Unable to create index: %s"
	// ReceiptStoreLevelDBConnect couldn't open file for the level DB
	ReceiptStoreLevelDBConnect = "Unable to open LevelDB: %s"

	// LevelDBFailedRetriveOriginalKey problem retrieving entry - original key
	LevelDBFailedRetriveOriginalKey = "Failed to retrieve the entry for the original key: %s. %s"
	// LevelDBFailedRetriveGeneratedID problem retrieving entry - generated ID
	LevelDBFailedRetriveGeneratedID = "Failed to retrieve the entry for the generated ID: %s. %s"

	// KVStoreDBLoad failed to init DB
	KVStoreDBLoad = "Failed to open DB at %s: %s"
	// KVStoreMemFilteringUnsupported memory db is really just for testing. No filtering support
	KVStoreMemFilteringUnsupported = "Memory receipts do not support filtering"

	// Unauthorized (401 error)
	Unauthorized = "Unauthorized"

	// EventStreamsWebSocketErrorFromClient Error message received from client
	EventStreamsWebSocketErrorFromClient = "Error received from WebSocket client: %s"
)

type RestErrMsg struct {
	Message string `json:"error"`
}

func RestErrReply(res http.ResponseWriter, req *http.Request, err error, status int) {
	log.Errorf("<-- %s %s [%d]: %s", req.Method, req.URL, status, err)
	reply, _ := json.Marshal(&RestErrMsg{Message: err.Error()})
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(status)
	_, _ = res.Write(reply)
}
