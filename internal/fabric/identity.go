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

package fabric

import (
	"encoding/json"
	"net/http"

	"github.com/hyperledger-labs/firefly-fabconnect/internal/rest/identity"
	restutil "github.com/hyperledger-labs/firefly-fabconnect/internal/rest/utils"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	mspApi "github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type identityManagerProvider struct {
	identityManager msp.IdentityManager
}

// IdentityManager returns the organization's identity manager
func (p *identityManagerProvider) IdentityManager(orgName string) (msp.IdentityManager, bool) {
	return p.identityManager, true
}

// the rpcWrapper is also an implementation of the interface internal/rest/idenity/IdentityClient
func (w *rpcWrapper) Register(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RegisterResponse, *restutil.RestError) {
	regreq := identity.Identity{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&regreq)
	if err != nil {
		return nil, restutil.NewRestError("failed to decode JSON payload", 400)
	}
	if regreq.Name == "" {
		return nil, restutil.NewRestError(`missing required parameter "name"`, 400)
	}
	if regreq.Type == "" {
		regreq.Type = "client"
	}

	rr := &mspApi.RegistrationRequest{
		Name:           regreq.Name,
		Type:           regreq.Type,
		MaxEnrollments: regreq.MaxEnrollments,
		Affiliation:    regreq.Affiliation,
		CAName:         regreq.CAName,
		Secret:         regreq.Secret,
	}
	secret, err := w.caClient.Register(rr)
	if err != nil {
		log.Errorf("Failed to register user %s. %s", regreq.Name, err)
		return nil, restutil.NewRestError(err.Error())
	}

	result := identity.RegisterResponse{
		Name:   regreq.Name,
		Secret: secret,
	}
	return &result, nil
}

func (w *rpcWrapper) Enroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.IdentityResponse, *restutil.RestError) {
	username := params.ByName("username")
	enreq := identity.EnrollRequest{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&enreq)
	if err != nil {
		return nil, restutil.NewRestError("failed to decode JSON payload", 400)
	}
	if enreq.Secret == "" {
		return nil, restutil.NewRestError(`missing required parameter "secret"`, 400)
	}

	input := mspApi.EnrollmentRequest{
		Name:    username,
		Secret:  enreq.Secret,
		CAName:  enreq.CAName,
		Profile: enreq.Profile,
	}

	err = w.caClient.Enroll(&input)
	if err != nil {
		log.Errorf("Failed to enroll user %s. %s", enreq.Name, err)
		return nil, restutil.NewRestError(err.Error())
	}

	result := identity.IdentityResponse{
		Name:    enreq.Name,
		Success: true,
	}
	return &result, nil
}

func (w *rpcWrapper) List(res http.ResponseWriter, req *http.Request, params httprouter.Params) ([]*identity.Identity, *restutil.RestError) {
	result, err := w.caClient.GetAllIdentities(params.ByName("caname"))
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	ret := make([]*identity.Identity, len(result))
	for i, v := range result {
		newId := identity.Identity{}
		newId.Name = v.ID
		newId.MaxEnrollments = v.MaxEnrollments
		newId.CAName = v.CAName
		newId.Type = v.Type
		newId.Affiliation = v.Affiliation
		ret[i] = &newId
	}
	return ret, nil
}

func (w *rpcWrapper) Get(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.Identity, *restutil.RestError) {
	username := params.ByName("username")
	result, err := w.caClient.GetIdentity(username, params.ByName("caname"))
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}

	// the SDK identity manager does not persist the certificates
	// we have to retrieve it from the identity manager
	si, _ := w.identityMgr.GetSigningIdentity(username)
	var ecert []byte
	var mspId string
	if si != nil {
		// the user may have been enrolled by a different client instance
		ecert = si.EnrollmentCertificate()
		mspId = si.Identifier().MSPID
	}

	// the SDK doesn't save the CACert locally, we have to retrieve it from the Fabric CA server
	cacert, err := w.getCACert()
	if err != nil {
		return nil, restutil.NewRestError(err.Error())
	}

	newId := identity.Identity{}
	newId.Name = result.ID
	newId.MaxEnrollments = result.MaxEnrollments
	newId.CAName = result.CAName
	newId.Type = result.Type
	newId.Affiliation = result.Affiliation
	newId.MSPID = mspId
	newId.EnrollmentCert = ecert
	newId.CACert = cacert
	return &newId, nil
}

func (w *rpcWrapper) getCACert() ([]byte, error) {
	result, err := w.caClient.GetCAInfo()
	if err != nil {
		log.Errorf("Failed to retrieve Fabric CA information: %s", err)
		return nil, err
	}
	return result.CAChain, nil
}
