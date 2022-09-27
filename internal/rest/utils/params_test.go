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

var payloadPinBatchAsString = `{
	"headers": {
		"type": "",
		"payloadSchema": {
			"type": "array",
			"prefixItems": [
				{
					"name": "uuids",
					"type": "string"
				},
				{
					"name": "batchHash",
					"type": "string"
				},
				{
					"name": "payloadRef",
					"type": "string"
				},
				{
					"name": "contexts",
					"type": "string"
				}
			]
		},
		"signer": "signer001",
		"channel": "firefly",
		"chaincode": "simplestorage"
	},
	"func": "PinBatch",
	"args": {
		"batchHash": "0x310a7b9a570eee114f7eb911c914a76c553541a43cfcb7da5f02a01fcba917e6",
		"contexts": "[\"0x2f87013b99fff4d8083acb8c6b9dff4e045cf58a8a3383201a6c7a1f2f4e71be\",\"0xb9deaaf95ad150198f074a058980bef204a307d712e3b484ee4210b3cd0a491b\"]",
		"payloadRef": "Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD",
		"uuids": "0x9ffc50ff6bfe4502adc793aea54cc059c5df767cfe444e038eb51c5523097db5"
	}
}`

var payloadTypesCheck = `{
	"headers": {
		"type": "",
		"payloadSchema": {
			"type": "array",
			"prefixItems": [
				{
					"name": "string",
					"type": "string"
				},
				{
					"name": "number",
					"type": "number"
				},
				{
					"name": "integer",
					"type": "integer"
				},
				{
					"name": "boolean",
					"type": "boolean"
				},
				{
					"name": "stringArray",
					"type": "array",
					"items": {
						"type": "string"
					}
				},
				{
					"name": "numberArray",
					"type": "array",
					"items": {
						"type": "number"
					}
				},
				{
					"name": "integerArray",
					"type": "array",
					"items": {
						"type": "number"
					}
				},
				{
					"name": "booleanArray",
					"type": "array",
					"items": {
						"type": "boolean"
					}
				}
			]
		}
	},
	"func": "TestTypes",
	"args": {
		"string": "abc",
		"number": "3.14",
		"integer": "42",
		"boolean": "true",
		"stringArray": "[\"def\",\"ghi\"]",
		"numberArray": "[3.14,6.28,9.42]",
		"integerArray": "[0,1,1,2,3,5,8]",
		"booleanArray": "[true,false,true,false]"
	}
}`

var payloadTypesInvalidInteger = `{
	"headers": {
		"type": "",
		"payloadSchema": {
			"type": "array",
			"prefixItems": [
				{
					"name": "invalidInteger",
					"type": "integer"
				}
			]
		}
	},
	"func": "TestTypes",
	"args": {
		"invalidInteger": "3.14"
	}
}`

var payloadTypesInvalidStringArray = `{
	"headers": {
		"type": "",
		"payloadSchema": {
			"type": "array",
			"prefixItems": [
				{
					"name": "invalidStringArray",
					"type": "array",
					"items": {
						"type": "string"
					}
				}
			]
		}
	},
	"func": "TestTypes",
	"args": {
		"invalidStringArray": "[\"def\",123,\"ghi\"]"
	}
}`

var payloadTypesInvalidNumberArray = `{
	"headers": {
		"type": "",
		"payloadSchema": {
			"type": "array",
			"prefixItems": [
				{
					"name": "invalidNumberArray",
					"type": "array",
					"items": {
						"type": "number"
					}
				}
			]
		}
	},
	"func": "TestTypes",
	"args": {
		"invalidNumberArray": "[3.14,\"foo\",9.42]"
	}
}`

var payloadTypesInvalidIntegerArray = `{
	"headers": {
		"type": "",
		"payloadSchema": {
			"type": "array",
			"prefixItems": [
				{
					"name": "invalidIntegerArray",
					"type": "array",
					"items": {
						"type": "integer"
					}
				}
			]
		}
	},
	"func": "TestTypes",
	"args": {
		"invalidIntegerArray": "[0,1,1,\"bar\",3,5,8]"
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

func TestProcessArgsPinBatchAsString(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadPinBatchAsString), &body)

	args, err := processArgs(body)
	assert.NoError(err)
	assert.Equal(args, []string{
		"0x9ffc50ff6bfe4502adc793aea54cc059c5df767cfe444e038eb51c5523097db5",
		"0x310a7b9a570eee114f7eb911c914a76c553541a43cfcb7da5f02a01fcba917e6",
		"Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD",
		"[\"0x2f87013b99fff4d8083acb8c6b9dff4e045cf58a8a3383201a6c7a1f2f4e71be\",\"0xb9deaaf95ad150198f074a058980bef204a307d712e3b484ee4210b3cd0a491b\"]",
	})
}

func TestProcessArgsPinBatchAsArray(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadPinBatchAsString), &body)

	args, err := processArgs(body)
	assert.NoError(err)
	assert.Equal(args, []string{
		"0x9ffc50ff6bfe4502adc793aea54cc059c5df767cfe444e038eb51c5523097db5",
		"0x310a7b9a570eee114f7eb911c914a76c553541a43cfcb7da5f02a01fcba917e6",
		"Qmf412jQZiuVUtdgnB36FXFX7xg5V6KEbSJ4dpQuhkLyfD",
		"[\"0x2f87013b99fff4d8083acb8c6b9dff4e045cf58a8a3383201a6c7a1f2f4e71be\",\"0xb9deaaf95ad150198f074a058980bef204a307d712e3b484ee4210b3cd0a491b\"]",
	})
}

func TestProcessArgsTypes(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadTypesCheck), &body)

	args, err := processArgs(body)
	assert.NoError(err)
	assert.Equal(args, []string{
		"abc",
		"3.14",
		"42",
		"true",
		"[\"def\",\"ghi\"]",
		"[3.14,6.28,9.42]",
		"[0,1,1,2,3,5,8]",
		"[true,false,true,false]",
	})
}

func TestProcessArgsTypesInvalidInteger(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadTypesInvalidInteger), &body)

	_, err := processArgs(body)
	assert.ErrorContains(err, "Expected: integer, given: number")
}

func TestProcessArgsTypesInvalidStringArray(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadTypesInvalidStringArray), &body)

	_, err := processArgs(body)
	assert.ErrorContains(err, "Expected: string, given: integer")
}

func TestProcessArgsTypesInvalidNumberArray(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadTypesInvalidNumberArray), &body)

	_, err := processArgs(body)
	assert.ErrorContains(err, "Expected: number, given: string")
}

func TestProcessArgsTypesInvalidIntegerArray(t *testing.T) {
	assert := assert.New(t)
	body := make(map[string]interface{})
	_ = json.Unmarshal([]byte(payloadTypesInvalidIntegerArray), &body)

	_, err := processArgs(body)
	assert.ErrorContains(err, "Expected: integer, given: string")
}
