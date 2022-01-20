// Copyright 2022 Kaleido

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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodingArrayOfJson(t *testing.T) {
	assert := assert.New(t)

	testBytes := []byte(`[{"a":1, "b":2}, {"a":10, "b":20}]`)
	result := DecodePayload(testBytes)
	m, ok := result.([]interface{})
	assert.Equal(true, ok)
	n, ok := m[0].(map[string]interface{})
	assert.Equal(true, ok)
	assert.Equal(float64(1), n["a"])
}

func TestDecodingJson(t *testing.T) {
	assert := assert.New(t)

	testBytes := []byte(`{"a":1, "b":2}`)
	result := DecodePayload(testBytes)
	m, ok := result.(map[string]interface{})
	assert.Equal(true, ok)
	assert.Equal(float64(1), m["a"])
}

func TestDecodingArrayOfStrings(t *testing.T) {
	assert := assert.New(t)

	testBytes := []byte(`["string1", "string2"]`)
	result := DecodePayload(testBytes)
	m, ok := result.([]interface{})
	assert.Equal(true, ok)
	n, ok := m[0].(string)
	assert.Equal(true, ok)
	assert.Equal("string1", n)
}

func TestDecodingArrayOfNumbers(t *testing.T) {
	assert := assert.New(t)

	testBytes := []byte(`[1, 2, 3]`)
	result := DecodePayload(testBytes)
	m, ok := result.([]interface{})
	assert.Equal(true, ok)
	n, ok := m[0].(float64)
	assert.Equal(true, ok)
	assert.Equal(float64(1), n)
}
