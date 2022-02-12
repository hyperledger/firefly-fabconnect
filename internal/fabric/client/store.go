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

package client

import (
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	"github.com/hyperledger/fabric-sdk-go/pkg/fab/keyvaluestore"
	mspImpl "github.com/hyperledger/fabric-sdk-go/pkg/msp"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
)

func newUserstore(configProvider core.ConfigProvider) (msp.UserStore, error) {
	configBackend, _ := configProvider()
	identityConfig, err := mspImpl.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, errors.Errorf("Failed to load identity configurations: %s", err)
	}
	clientConfig := identityConfig.Client()
	if clientConfig.CredentialStore.Path == "" {
		return nil, errors.Errorf("User credentials store path is empty")
	}
	store, err := keyvaluestore.New(&keyvaluestore.FileKeyValueStoreOptions{
		Path: clientConfig.CredentialStore.Path,
	})
	if err != nil {
		return nil, errors.Errorf("Key-value store creation failed. %s", clientConfig.CredentialStore.Path)
	}
	userStore, err := mspImpl.NewCertFileUserStore1(store)
	if err != nil {
		return nil, errors.Errorf("User credentials store creation failed. %s", err)
	}
	return userStore, nil
}
