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

import "fmt"

type RestError struct {
	Error      error
	StatusCode int
}

func NewRestError(msg string, code ...int) *RestError {
	statusCode := 500
	if len(code) > 0 {
		statusCode = code[0]
	}
	return &RestError{
		Error:      fmt.Errorf(msg),
		StatusCode: statusCode,
	}
}
