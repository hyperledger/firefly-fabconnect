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

package utils

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"strings"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/errors"
	"github.com/icza/dyno"
	uuid "github.com/nu7hatch/gouuid"
	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
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

// YAMLorJSONPayload processes either a YAML or JSON payload from an input HTTP request
func YAMLorJSONPayload(req *http.Request) (map[string]interface{}, error) {

	if req.ContentLength > MaxPayloadSize {
		return nil, errors.Errorf(errors.HelperYAMLorJSONPayloadTooLarge)
	}
	originalPayload, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, errors.Errorf(errors.HelperYAMLorJSONPayloadReadFailed, err)
	}

	// We support both YAML and JSON input.
	// We parse the message into a generic string->interface map, that lets
	// us check a couple of routing fields needed to dispatch the messages
	// to Kafka (always in JSON). However, we do not perform full parsing.
	var msg map[string]interface{}
	contentType := strings.ToLower(req.Header.Get("Content-type"))
	log.Infof("Received message 'Content-Type: %s' Length: %d", contentType, req.ContentLength)
	if req.ContentLength == 0 {
		return map[string]interface{}{}, nil
	}

	// Unless explicitly declared as YAML, try JSON first
	var unmarshalledAsJSON = false
	if contentType != "application/x-yaml" && contentType != "text/yaml" {
		err := json.Unmarshal(originalPayload, &msg)
		if err != nil {
			log.Debugf("Payload is not valid JSON - trying YAML: %s", err)
		} else {
			unmarshalledAsJSON = true
		}
	}
	// Try YAML if content-type is set, or if JSON fails
	if !unmarshalledAsJSON {
		yamlGenericPayload := make(map[interface{}]interface{})
		err := yaml.Unmarshal(originalPayload, &yamlGenericPayload)
		if err != nil {
			return nil, errors.Errorf(errors.HelperYAMLorJSONPayloadParseFailed, err)
		}
		msg = dyno.ConvertMapI2MapS(yamlGenericPayload).(map[string]interface{})
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
