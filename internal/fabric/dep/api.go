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
package dep

import (
	mspApi "github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
)

type CAClient interface {
	Register(*mspApi.RegistrationRequest) (string, error)
	ModifyIdentity(*mspApi.IdentityRequest) (*mspApi.IdentityResponse, error)
	Enroll(*mspApi.EnrollmentRequest) error
	Reenroll(*mspApi.ReenrollmentRequest) error
	Revoke(*mspApi.RevocationRequest) (*mspApi.RevocationResponse, error)
	GetAllIdentities(string) ([]*mspApi.IdentityResponse, error)
	GetIdentity(string, string) (*mspApi.IdentityResponse, error)
	GetCAInfo() (*mspApi.GetCAInfoResponse, error)
}
