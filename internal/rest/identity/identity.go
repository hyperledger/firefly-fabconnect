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

package identity

import (
	"net/http"

	restutil "github.com/hyperledger/firefly-fabconnect/internal/rest/utils"
	"github.com/julienschmidt/httprouter"
)

type Identity struct {
	RegisterResponse
	MaxEnrollments int               `json:"maxEnrollments"`
	Type           string            `json:"type"`
	Affiliation    string            `json:"affiliation"`
	Attributes     map[string]string `json:"attributes"`
	CAName         string            `json:"caname"`
	Organization   string            `json:"organization,omitempty"`
	MSPID          string            `json:"mspId,omitempty"`
	EnrollmentCert []byte            `json:"enrollmentCert,omitempty"`
	CACert         []byte            `json:"caCert,omitempty"`
}

type RegisterResponse struct {
	Name   string `json:"name"`
	Secret string `json:"secret,omitempty"`
}

type EnrollRequest struct {
	Name     string          `json:"name"`
	Secret   string          `json:"secret"`
	CAName   string          `json:"caname"`
	Profile  string          `json:"profile"`
	AttrReqs map[string]bool `json:"attributes"`
}

type RevokeRequest struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
	CAName string `json:"caname"`
	GenCRL bool   `json:"generateCRL"`
}

type IdentityResponse struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
}

type RevokeResponse struct {
	RevokedCerts []map[string]string `json:"revokedCerts"`
	CRL          []byte              `json:"CRL"`
}

type IdentityClient interface {
	Register(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*RegisterResponse, *restutil.RestError)
	Modify(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*RegisterResponse, *restutil.RestError)
	Enroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*IdentityResponse, *restutil.RestError)
	Reenroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*IdentityResponse, *restutil.RestError)
	Revoke(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*RevokeResponse, *restutil.RestError)
	List(res http.ResponseWriter, req *http.Request, params httprouter.Params) ([]*Identity, *restutil.RestError)
	Get(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*Identity, *restutil.RestError)
}
