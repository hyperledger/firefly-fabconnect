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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

var payloadNoSchema = `{
	"headers": {
			"type": "SendTransaction"
	},
	"func": "CreateAsset",
	"args": ["asset204", "red", "10", "Tom", "123000"]
}`

var payloadSimpleSchema = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
					"type": "array",
					"prefixItems": [{
							"name": "id", "type": "string"
					}, {
							"name": "color", "type": "string"
					}, {
							"name": "size", "type": "string"
					}, {
							"name": "owner", "type": "string"
					}, {
							"name": "value", "type": "string"
					}]
			}
	},
	"func": "CreateAsset",
	"args": {
			"owner": "Tom",
			"value": "123000",
			"size": "10",
			"id": "asset204",
			"color": "red"
	}
}`

var payloadComplexSchema = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
					"type": "array",
					"prefixItems": [{
							"name": "id", "type": "string"
					}, {
							"name": "color", "type": "string"
					}, {
							"name": "size", "type": "string"
					}, {
							"name": "owner", "type": "string"
					}, {
							"name": "appraisal", "type": "object",
							"properties": {
									"appraisedValue": {
											"type": "integer"
									},
									"inspected": {
											"type": "boolean"
									}
							}
					}]
			}
	},
	"func": "CreateAsset",
	"args": {
			"owner": "Tom",
			"appraisal": {
					"appraisedValue": 123000,
					"inspected": true
			},
			"size": "10",
			"id": "asset204",
			"color": "red"
	}
}`

var payloadNoArgs = `{
	"headers": {
			"type": "SendTransaction"
	},
	"func": "CreateAsset"
}`

var payloadArgsNotJSON = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
					"type": "array",
					"prefixItems": [{
							"name": "id", "type": "string"
					}, {
							"name": "color", "type": "string"
					}, {
							"name": "size", "type": "string"
					}, {
							"name": "owner", "type": "string"
					}, {
							"name": "value", "type": "string"
					}]
			}
	},
	"func": "CreateAsset",
	"args": ["asset204", "red", "10", "Tom", "123000"]
}`

var payloadArgsBadSchema1 = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": ["bad schema"]
	},
	"func": "CreateAsset",
	"args": {}
}`

var payloadArgsBadSchema2 = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
				"type": "object"
			}
	},
	"func": "CreateAsset",
	"args": {}
}`

var payloadArgsNotPrefixItems = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
					"type": "array",
					"items": [{
							"name": "id", "type": "string"
					}, {
							"name": "color", "type": "string"
					}, {
							"name": "size", "type": "string"
					}, {
							"name": "owner", "type": "string"
					}, {
							"name": "value", "type": "string"
					}]
			}
	},
	"func": "CreateAsset",
	"args": {}
}`

var payloadArgsItemsWithNoName = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
					"type": "array",
					"prefixItems": [{
							"type": "string"
					}]
			}
	},
	"func": "CreateAsset",
	"args": {}
}`

var payloadObjectNoSchema = `{
	"headers": {
			"type": "SendTransaction"
	},
	"func": "CreateAsset",
	"args": {}
}`

var payloadSimpleSchemaBadValue = `{
	"headers": {
			"type": "SendTransaction",
			"payloadSchema": {
					"type": "array",
					"prefixItems": [{
							"name": "id", "type": "string"
					}]
			}
	},
	"func": "CreateAsset",
	"args": {
			"id": 100
	}
}`

func TestProcessArgsNotSchema(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadNoSchema), &body)

	args, err := processArgs(body)
	assert.NoError(err)
	assert.Equal(args, []string{"asset204", "red", "10", "Tom", "123000"})
}

func TestProcessArgsSimpleSchema(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadSimpleSchema), &body)

	args, err := processArgs(body)
	assert.NoError(err)
	assert.Equal(args, []string{"asset204", "red", "10", "Tom", "123000"})
}

func TestProcessArgsComplexSchema(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadComplexSchema), &body)

	args, err := processArgs(body)
	assert.NoError(err)
	assert.Equal(args, []string{"asset204", "red", "10", "Tom", "{\"appraisedValue\":123000,\"inspected\":true}"})
}

func TestProcessArgsMissingArgs(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadNoArgs), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "must specify args")
}

func TestProcessArgsNotJSON(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadArgsNotJSON), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "the \"args\" property must be JSON if a payload schema is provided")
}

func TestProcessArgsBadSchema(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadArgsBadSchema1), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "invalid payload schema")
}

func TestProcessArgsWrongRootType(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadArgsBadSchema2), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "payload schema must define a root type of \"array\"")
}

func TestProcessArgsNotPrefixItems(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadArgsNotPrefixItems), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "payload schema must define a root type of \"array\" using \"prefixItems\"")
}

func TestProcessArgsItemsHaveNoName(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadArgsItemsWithNoName), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "property definitions of the \"prefixItems\" in the payload schema must have a \"name\"")
}

func TestProcessArgsObjectWithNoSchema(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadObjectNoSchema), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "no payload schema is specified in the payload's \"headers\", the \"args\" property must be an array of strings")
}

func TestProcessArgsBadValue(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadSimpleSchemaBadValue), &body)

	_, err := processArgs(body)
	assert.EqualError(err, "failed to validate argument \"id\": - (root): Invalid type. Expected: string, given: integer\n")
}
