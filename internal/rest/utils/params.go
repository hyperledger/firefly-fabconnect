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
	"strings"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/hyperledger-labs/firefly-fabconnect/internal/utils"
	"github.com/julienschmidt/httprouter"
)

func ResolveParams(res http.ResponseWriter, req *http.Request, params httprouter.Params) (c map[string]interface{}, err error) {
	// if query parameters are specified, they take precedence over body properties
	channelParam := params.ByName("channel")
	methodParam := params.ByName("func")
	signer := GetFlyParam("signer", req, false)
	blocknumber := GetFlyParam("blocknumber", req, false)

	body, err := utils.ParseJSONPayload(req)
	if err != nil {
		errors.RestErrReply(res, req, err, 400)
		return nil, err
	}

	// consolidate inidividual parameters with the body parameters
	if channelParam != "" {
		body["headers"].(map[string]string)["channel"] = channelParam
	}
	if methodParam != "" {
		body["func"] = methodParam
	}
	if signer != "" {
		body["headers"].(map[string]string)["signer"] = signer
	}
	if blocknumber != "" {
		body["headers"].(map[string]string)["blocknumber"] = blocknumber
	}

	return body, nil
}

// getFlyParam standardizes how special 'fly' params are specified, in query params, or headers
func GetFlyParam(name string, req *http.Request, isBool bool) string {
	valStr := ""
	vs := getQueryParamNoCase(utils.GetenvOrDefaultLowerCase("PREFIX_SHORT", "fly")+"-"+name, req)
	if len(vs) > 0 {
		valStr = vs[0]
	}
	if isBool && valStr == "" && len(vs) > 0 {
		valStr = "true"
	}
	if valStr == "" {
		valStr = req.Header.Get("x-" + utils.GetenvOrDefaultLowerCase("PREFIX_LONG", "firefly") + "-" + name)
	}
	return valStr
}

func getQueryParamNoCase(name string, req *http.Request) []string {
	name = strings.ToLower(name)
	_ = req.ParseForm()
	for k, vs := range req.Form {
		if strings.ToLower(k) == name {
			return vs
		}
	}
	return nil
}
