// Copyright 2022 Kaleido
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

package api

// ReceiptStorePersistence interface implemented by persistence layers
type ReceiptStorePersistence interface {
	Init() error
	ValidateConf() error
	GetReceipts(skip, limit int, ids []string, sinceEpochMS int64, from, to, start string) (*[]map[string]interface{}, error)
	GetReceipt(requestID string) (*map[string]interface{}, error)
	AddReceipt(requestID string, receipt *map[string]interface{}) error
	Close()
}
