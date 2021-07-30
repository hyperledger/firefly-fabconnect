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

package util

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/messages"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/utils"
	"github.com/julienschmidt/httprouter"
)

type TxOpts struct {
	Sync bool // synchronous request or not
	Ack  bool // expect acknowledgement from the async request or not
}

// getFlyParam standardizes how special 'fly' params are specified, in body, query params, or headers
// these fly-* parameters are supported:
//   - signer, channel, chaincode
// precedence order:
//   - "headers" in body > query parameters > http headers
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

func BuildTxMessage(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*messages.SendTransaction, *TxOpts, *RestError) {
	body, err := utils.ParseJSONPayload(req)
	if err != nil {
		return nil, nil, NewRestError(err.Error(), 400)
	}
	err = req.ParseForm()
	if err != nil {
		return nil, nil, NewRestError(err.Error(), 400)
	}

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
	msg.Headers.MsgType = messages.MsgTypeSendTransaction
	msg.Headers.ChannelID = channel
	msg.Headers.Signer = signer
	msg.Headers.ChaincodeName = chaincode
	isInitVal := body["init"]
	if isInitVal != nil {
		isInit, err := strconv.ParseBool(isInitVal.(string))
		if err != nil {
			return nil, nil, NewRestError(err.Error(), 400)
		}
		msg.IsInit = isInit
	}
	msg.Function = body["func"].(string)
	if msg.Function == "" {
		return nil, nil, NewRestError("Must specify target chaincode function", 400)
	}
	argsVal := body["args"]
	if argsVal == nil {
		return nil, nil, NewRestError("Must specify args", 400)
	}
	args := make([]string, len(argsVal.([]interface{})))
	for i, v := range argsVal.([]interface{}) {
		args[i] = v.(string)
	}
	msg.Args = args

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
