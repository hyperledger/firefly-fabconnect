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

package utils

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	uuid "github.com/nu7hatch/gouuid"
	"gopkg.in/yaml.v2"
)

const (
	// MaxPayloadSize max size of content
	MaxPayloadSize = 1024 * 1024
)

// AllOrNoneReqd util for checking parameters that must be provided together
func AllOrNoneReqd(opts ...string) (ok bool) {
	var setFlags, unsetFlags []string
	for _, opt := range opts {
		if opt != "" {
			setFlags = append(setFlags, opt)
		} else {
			unsetFlags = append(unsetFlags, opt)
		}
	}
	ok = !(len(setFlags) != 0 && len(unsetFlags) != 0)
	return
}

// MarshalToYAML marshals a JSON annotated structure into YAML, by first going to JSON
func MarshalToYAML(conf interface{}) (yamlBytes []byte, err error) {
	var jsonBytes []byte
	if jsonBytes, err = json.Marshal(conf); err != nil {
		return
	}
	jsonAsMap := make(map[string]interface{})
	if err = json.Unmarshal(jsonBytes, &jsonAsMap); err != nil {
		return
	}
	yamlBytes, err = yaml.Marshal(&jsonAsMap)
	return
}

// GetMapString is a helper to safely extract strings from generic interface maps
func GetMapString(genericMap map[string]interface{}, key string) string {
	if val, exists := genericMap[key]; exists {
		if reflect.TypeOf(val).Kind() == reflect.String {
			return val.(string)
		}
	}
	return ""
}

// UUIDv4 returns a new UUID V4 as a string
func UUIDv4() string {
	uuidV4, _ := uuid.NewV4()
	return uuidV4.String()
}

// parseJSONPayload processes either a YAML or JSON payload from an input HTTP request
func ParseJSONPayload(req *http.Request) (map[string]interface{}, error) {

	if req.ContentLength > MaxPayloadSize {
		return nil, errors.Errorf(errors.HelperPayloadTooLarge)
	}
	if req.ContentLength == 0 {
		return map[string]interface{}{}, nil
	}

	var msg map[string]interface{}
	err := json.NewDecoder(req.Body).Decode(&msg)
	if err != nil {
		return nil, errors.Errorf(errors.HelperPayloadParseFailed)
	}

	return msg, nil
}

func GetenvOrDefault(varName, defaultVal string) string {
	val := os.Getenv(varName)
	if val == "" {
		return defaultVal
	}
	return val
}

func GetenvOrDefaultUpperCase(varName, defaultVal string) string {
	val := GetenvOrDefault(varName, defaultVal)
	return strings.ToUpper(val)
}

func GetenvOrDefaultLowerCase(varName, defaultVal string) string {
	val := GetenvOrDefault(varName, defaultVal)
	return strings.ToLower(val)
}
