// Copyright © 2023 Kaleido, Inc.
//
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hyperledger/firefly-fabconnect/internal/messages"
	"github.com/hyperledger/firefly-fabconnect/internal/utils"
	"github.com/julienschmidt/httprouter"
	jsonschema "github.com/xeipuuv/gojsonschema"
)

type TxOpts struct {
	Sync bool // synchronous request or not
	Ack  bool // expect acknowledgement from the async request or not
}

// getFlyParam standardizes how special 'fly' params are specified, in body, query params, or headers
// these fly-* parameters are supported:
//   - signer, channel, chaincode
//
// precedence order:
//   - "headers" in body > query parameters > http headers
//
// naming conventions:
//   - properties in the "headers" section of the body has the original name, eg. "channel"
//   - query parameter has the short prefix, eg. "fly-channel"
//   - header has the long prefix, eg. "x-firefly-channel"
func getFlyParam(name string, body map[string]interface{}, req *http.Request) string {
	valStr := ""
	// first look inside the "headers" section in the body
	s := body["headers"]
	if s != nil {
		v := s.(map[string]interface{})[name]
		if v != nil {
			valStr = v.(string)
		}
	}
	// next look in the query params
	if valStr == "" {
		vs := getQueryParamNoCase(utils.GetenvOrDefaultLowerCase("PREFIX_SHORT", "fly")+"-"+name, req)
		if len(vs) > 0 {
			valStr = vs[0]
		}
	}
	// finally look inside headers
	if valStr == "" {
		valStr = req.Header.Get("x-" + utils.GetenvOrDefaultLowerCase("PREFIX_LONG", "firefly") + "-" + name)
	}
	return valStr
}

func getQueryParamNoCase(name string, req *http.Request) []string {
	name = strings.ToLower(name)
	for k, vs := range req.Form {
		if strings.ToLower(k) == name {
			return vs
		}
	}
	return nil
}

func BuildQueryMessage(_ http.ResponseWriter, req *http.Request, _ httprouter.Params) (*messages.QueryChaincode, *RestError) {
	body, err := utils.ParseJSONPayload(req)
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}
	err = req.ParseForm()
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}

	msgID := getFlyParam("id", body, req)
	channel := getFlyParam("channel", body, req)
	if channel == "" {
		return nil, NewRestError("Must specify the channel", 400)
	}
	signer := getFlyParam("signer", body, req)
	if signer == "" {
		return nil, NewRestError("Must specify the signer", 400)
	}
	chaincode := getFlyParam("chaincode", body, req)
	if chaincode == "" {
		return nil, NewRestError("Must specify the chaincode name", 400)
	}

	msg := messages.QueryChaincode{}
	msg.Headers.ID = msgID // this could be empty
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer
	msg.Headers.ChaincodeName = chaincode
	if body["func"] == nil {
		return nil, NewRestError("Must specify target chaincode function", 400)
	}
	msg.Function = body["func"].(string)
	if msg.Function == "" {
		return nil, NewRestError("Target chaincode function must not be empty", 400)
	}
	argsVal, err := processArgs(body)
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}
	msg.Args = argsVal
	strongread := body["strongread"]
	if strongread != nil {
		strVal, ok := strongread.(string)
		if ok {
			isStrongread, err := strconv.ParseBool(strVal)
			if err != nil {
				return nil, NewRestError(err.Error(), 400)
			}
			msg.StrongRead = isStrongread
		} else {
			msg.StrongRead = strongread.(bool)
		}
	}

	return &msg, nil
}

func BuildTxByIDMessage(_ http.ResponseWriter, req *http.Request, params httprouter.Params) (*messages.GetTxByID, *RestError) {
	var body map[string]interface{}
	err := req.ParseForm()
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}
	msgID := getFlyParam("id", body, req)
	channel := getFlyParam("channel", body, req)
	if channel == "" {
		return nil, NewRestError("Must specify the channel", 400)
	}
	signer := getFlyParam("signer", body, req)
	if signer == "" {
		return nil, NewRestError("Must specify the signer", 400)
	}

	msg := messages.GetTxByID{}
	msg.Headers.ID = msgID // this could be empty
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer
	msg.TxID = params.ByName("txId")

	return &msg, nil
}

func BuildGetChainInfoMessage(_ http.ResponseWriter, req *http.Request, _ httprouter.Params) (*messages.GetChainInfo, *RestError) {
	var body map[string]interface{}
	err := req.ParseForm()
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}
	msgID := getFlyParam("id", body, req)
	channel := getFlyParam("channel", body, req)
	if channel == "" {
		return nil, NewRestError("Must specify the channel", 400)
	}
	signer := getFlyParam("signer", body, req)
	if signer == "" {
		return nil, NewRestError("Must specify the signer", 400)
	}

	msg := messages.GetChainInfo{}
	msg.Headers.ID = msgID // this could be empty
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer

	return &msg, nil
}

func BuildGetBlockMessage(_ http.ResponseWriter, req *http.Request, params httprouter.Params) (*messages.GetBlock, *RestError) {
	var body map[string]interface{}
	err := req.ParseForm()
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}
	msgID := getFlyParam("id", body, req)
	channel := getFlyParam("channel", body, req)
	if channel == "" {
		return nil, NewRestError("Must specify the channel", 400)
	}
	signer := getFlyParam("signer", body, req)
	if signer == "" {
		return nil, NewRestError("Must specify the signer", 400)
	}

	msg := messages.GetBlock{}
	msg.Headers.ID = msgID // this could be empty
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer

	blockNumberOrHash := params.ByName("blockNumber")
	if len(blockNumberOrHash) == 64 {
		// 32-byte hex string means this is a block hash
		bytes, err := hex.DecodeString(blockNumberOrHash)
		if err != nil {
			return nil, NewRestError("Invalid block hash", 400)
		}
		msg.BlockHash = bytes
	} else {
		blockNumber, err := strconv.ParseUint(blockNumberOrHash, 10, 64)
		if err != nil {
			return nil, NewRestError("Invalid block number", 400)
		}
		msg.BlockNumber = blockNumber
	}

	return &msg, nil
}

func BuildGetBlockByTxIDMessage(_ http.ResponseWriter, req *http.Request, params httprouter.Params) (*messages.GetBlockByTxID, *RestError) {
	var body map[string]interface{}
	err := req.ParseForm()
	if err != nil {
		return nil, NewRestError(err.Error(), 400)
	}
	channel := getFlyParam("channel", body, req)
	if channel == "" {
		return nil, NewRestError("Must specify the channel", 400)
	}
	signer := getFlyParam("signer", body, req)
	if signer == "" {
		return nil, NewRestError("Must specify the signer", 400)
	}

	msg := messages.GetBlockByTxID{}
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer
	msg.TxID = params.ByName("txId")

	return &msg, nil
}

func BuildTxMessage(_ http.ResponseWriter, req *http.Request, _ httprouter.Params) (*messages.SendTransaction, *TxOpts, *RestError) {
	body, err := utils.ParseJSONPayload(req)
	if err != nil {
		return nil, nil, NewRestError(err.Error(), 400)
	}
	err = req.ParseForm()
	if err != nil {
		return nil, nil, NewRestError(err.Error(), 400)
	}

	msgID := getFlyParam("id", body, req)
	channel := getFlyParam("channel", body, req)
	if channel == "" {
		return nil, nil, NewRestError("Must specify the channel", 400)
	}
	signer := getFlyParam("signer", body, req)
	if signer == "" {
		return nil, nil, NewRestError("Must specify the signer", 400)
	}
	chaincode := getFlyParam("chaincode", body, req)
	if chaincode == "" {
		return nil, nil, NewRestError("Must specify the chaincode name", 400)
	}

	msg := messages.SendTransaction{}
	msg.Headers.ID = msgID // this could be empty
	msg.Headers.MsgType = messages.MsgTypeSendTransaction
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer
	msg.Headers.ChaincodeName = chaincode
	isInitVal := body["init"]
	if isInitVal != nil {
		strVal, ok := isInitVal.(string)
		if ok {
			isInit, err := strconv.ParseBool(strVal)
			if err != nil {
				return nil, nil, NewRestError(err.Error(), 400)
			}
			msg.IsInit = isInit
		} else {
			msg.IsInit = isInitVal.(bool)
		}
	}
	msg.Function = body["func"].(string)
	if msg.Function == "" {
		return nil, nil, NewRestError("Must specify target chaincode function", 400)
	}
	argsVal, err := processArgs(body)
	if err != nil {
		return nil, nil, NewRestError(err.Error(), 400)
	}
	msg.Args = argsVal
	transientMap := body["transientMap"]
	if transientMap != nil {
		tmpMap := transientMap.(map[string]interface{})
		msg.TransientMap = make(map[string]string, len(tmpMap))
		for k, v := range tmpMap {
			msg.TransientMap[k] = v.(string)
		}
	}

	opts := TxOpts{}
	opts.Sync = true
	opts.Ack = true
	syncVal := getFlyParam("sync", body, req)
	if syncVal != "" {
		sync, err := strconv.ParseBool(syncVal)
		if err != nil {
			return nil, nil, NewRestError(err.Error(), 400)
		}
		opts.Sync = sync
	}
	noAckVal := getFlyParam("noack", body, req)
	if noAckVal != "" {
		noack, err := strconv.ParseBool(noAckVal)
		if err != nil {
			return nil, nil, NewRestError(err.Error(), 400)
		}
		opts.Ack = !noack
	}

	return &msg, &opts, nil
}

func processArgs(body map[string]interface{}) ([]string, error) {
	var args []string
	argsVal := body["args"]
	if argsVal == nil {
		return nil, fmt.Errorf("must specify args")
	}

	var payloadSchema interface{}
	headers := body["headers"]
	if headers != nil {
		payloadSchema = headers.(map[string]interface{})["payloadSchema"]
	}
	if payloadSchema != nil {
		// if a payload schema is provided, the args property in the body must be a JSON structure
		argsMap, ok := argsVal.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("the \"args\" property must be JSON if a payload schema is provided")
		}

		// the payload JSON schema can be supplied as a stringified JSON or expanded JSON
		var schemaloader jsonschema.JSONLoader
		schemaStr, ok := payloadSchema.(string)
		if ok {
			schemaloader = jsonschema.NewStringLoader(schemaStr)
		} else {
			schemaMap, ok := payloadSchema.(map[string]interface{})
			if ok {
				schemaloader = jsonschema.NewGoLoader(schemaMap)
			} else {
				return nil, fmt.Errorf("invalid payload schema")
			}
		}

		// the schema must specify an array root property
		schema, err := schemaloader.LoadJSON()
		if err != nil {
			return nil, err
		}
		schemaMap := schema.(map[string]interface{})
		rootType := schemaMap["type"]
		if rootType.(string) != "array" {
			return nil, fmt.Errorf("payload schema must define a root type of \"array\"")
		}
		// we require the schema to use "prefixItems" to define the ordered array of arguments
		pitems := schemaMap["prefixItems"]
		if pitems == nil {
			return nil, fmt.Errorf("payload schema must define a root type of \"array\" using \"prefixItems\"")
		}

		// only one level of property definitions are needed in order to understand
		// how to validate the args and marshal them into the array of strings to pass into Fabric
		items := pitems.([]interface{})
		args = make([]string, len(items))
		for i, item := range items {
			itemDef := item.(map[string]interface{})
			name := itemDef["name"]
			if name == nil {
				return nil, fmt.Errorf("property definitions of the \"prefixItems\" in the payload schema must have a \"name\"")
			}
			entry := argsMap[name.(string)]

			var entryStringValue string

			propType := itemDef["type"].(string)

			// If the type isn't string or object then it needs to be unmarshalled from JSON format
			if propType != "string" && propType != "object" {
				entryStringValue, ok = entry.(string)

				if !ok {
					return nil, errors.New("invalid object passed")
				}

				err := json.Unmarshal([]byte(entryStringValue), &entry)

				if err != nil {
					return nil, err
				}
			}

			// validate the args entry against the schema, matching by "name"
			err := validate(itemDef, name.(string), entry)
			if err != nil {
				return nil, err
			}

			switch propType {
			case "object":
				encoded, _ := json.Marshal(entry)
				args[i] = string(encoded)
			case "string":
				strVal, ok := entry.(string)
				if !ok {
					return nil, fmt.Errorf("argument property %q of type %q could not be converted to a string", name, propType)
				}
				args[i] = strVal
			default:
				args[i] = entryStringValue
			}
		}
	} else {
		// no payload schema provided, treat the args as array of strings
		argVals, ok := argsVal.([]interface{})
		if !ok {
			return nil, fmt.Errorf("no payload schema is specified in the payload's \"headers\", the \"args\" property must be an array of strings")
		}
		args = make([]string, len(argVals))
		for i, v := range argsVal.([]interface{}) {
			args[i] = v.(string)
		}
	}

	return args, nil
}

func validate(def map[string]interface{}, name string, value interface{}) error {
	// let's use the schema to validate the body args payload first
	document := jsonschema.NewGoLoader(value)
	schemaloader := jsonschema.NewGoLoader(def)
	result, err := jsonschema.Validate(schemaloader, document)
	if err != nil {
		return fmt.Errorf("failed to validate argument \"%s\": %s", name, err)
	}
	if !result.Valid() {
		errorMsg := ""
		for _, desc := range result.Errors() {
			errorMsg = fmt.Sprintf("%s- %s\n", errorMsg, desc)
		}
		return fmt.Errorf("failed to validate argument \"%s\": %s", name, errorMsg)
	}
	return nil
}
