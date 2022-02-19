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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/core"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/msp"
	fabcontext "github.com/hyperledger/fabric-sdk-go/pkg/context"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/cryptosuite/bccsp/sw"
	fabImpl "github.com/hyperledger/fabric-sdk-go/pkg/fab"
	mspImpl "github.com/hyperledger/fabric-sdk-go/pkg/msp"
	mspApi "github.com/hyperledger/fabric-sdk-go/pkg/msp/api"
	"github.com/hyperledger/firefly-fabconnect/internal/errors"
	"github.com/hyperledger/firefly-fabconnect/internal/rest/identity"
	restutil "github.com/hyperledger/firefly-fabconnect/internal/rest/utils"
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

type idClientWrapper struct {
	identityConfig msp.IdentityConfig
	identityMgr    msp.IdentityManager
	caClient       mspApi.CAClient
}

func newIdentityClient(configProvider core.ConfigProvider, userStore msp.UserStore) (*idClientWrapper, error) {
	configBackend, _ := configProvider()
	cryptoConfig := cryptosuite.ConfigFromBackend(configBackend...)
	cs, err := sw.GetSuiteByConfig(cryptoConfig)
	if err != nil {
		return nil, errors.Errorf("Failed to get suite by config: %s", err)
	}
	endpointConfig, err := fabImpl.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, errors.Errorf("Failed to read config: %s", err)
	}
	identityConfig, err := mspImpl.ConfigFromBackend(configBackend...)
	if err != nil {
		return nil, errors.Errorf("Failed to load identity configurations: %s", err)
	}
	clientConfig := identityConfig.Client()
	if clientConfig.CredentialStore.Path == "" {
		return nil, errors.Errorf("User credentials store path is empty")
	}
	mgr, err := mspImpl.NewIdentityManager(clientConfig.Organization, userStore, cs, endpointConfig)
	if err != nil {
		return nil, errors.Errorf("Identity manager creation failed. %s", err)
	}

	identityManagerProvider := &identityManagerProvider{
		identityManager: mgr,
	}
	ctxProvider := fabcontext.NewProvider(
		fabcontext.WithIdentityManagerProvider(identityManagerProvider),
		fabcontext.WithUserStore(userStore),
		fabcontext.WithCryptoSuite(cs),
		fabcontext.WithCryptoSuiteConfig(cryptoConfig),
		fabcontext.WithEndpointConfig(endpointConfig),
		fabcontext.WithIdentityConfig(identityConfig),
	)
	ctx := &fabcontext.Client{
		Providers: ctxProvider,
	}
	caClient, err := mspImpl.NewCAClient(clientConfig.Organization, ctx)
	if err != nil {
		return nil, errors.Errorf("CA Client creation failed. %s", err)
	}
	idc := &idClientWrapper{
		identityConfig: identityConfig,
		identityMgr:    mgr,
		caClient:       caClient,
	}
	return idc, nil
}

func (w *idClientWrapper) GetSigningIdentity(name string) (msp.SigningIdentity, error) {
	return w.identityMgr.GetSigningIdentity(name)
}

func (w *idClientWrapper) GetClientOrg() string {
	return w.identityConfig.Client().Organization
}

// the rpcWrapper is also an implementation of the interface internal/rest/idenity/IdentityClient
func (w *idClientWrapper) Register(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RegisterResponse, *restutil.RestError) {
	regreq := identity.Identity{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&regreq)
	if err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf("failed to decode JSON payload: %s", err), 400)
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
	if regreq.Attributes != nil {
		rr.Attributes = []mspApi.Attribute{}
		for key, value := range regreq.Attributes {
			rr.Attributes = append(rr.Attributes, mspApi.Attribute{Name: key, Value: value})
		}
	}

	secret, err := w.caClient.Register(rr)
	if err != nil {
		log.Errorf("Failed to register user %s. %s", regreq.Name, err)
		return nil, restutil.NewRestError(err.Error())
	}

	result := identity.RegisterResponse{
		Name:   rr.Name,
		Secret: secret,
	}
	return &result, nil
}

func (w *idClientWrapper) Modify(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RegisterResponse, *restutil.RestError) {
	username := params.ByName("username")
	regreq := identity.Identity{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&regreq)
	if err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf("failed to decode JSON payload: %s", err), 400)
	}

	rr := &mspApi.IdentityRequest{
		ID:             username,
		Type:           regreq.Type,
		MaxEnrollments: regreq.MaxEnrollments,
		Affiliation:    regreq.Affiliation,
		CAName:         regreq.CAName,
		Secret:         regreq.Secret,
	}
	if regreq.Attributes != nil {
		rr.Attributes = []mspApi.Attribute{}
		for key, value := range regreq.Attributes {
			rr.Attributes = append(rr.Attributes, mspApi.Attribute{Name: key, Value: value})
		}
	}

	_, err = w.caClient.ModifyIdentity(rr)
	if err != nil {
		log.Errorf("Failed to modify user %s. %s", regreq.Name, err)
		return nil, restutil.NewRestError(err.Error())
	}

	result := identity.RegisterResponse{
		Name: rr.ID,
	}
	return &result, nil
}

func (w *idClientWrapper) Enroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.IdentityResponse, *restutil.RestError) {
	username := params.ByName("username")
	enreq := identity.EnrollRequest{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&enreq)
	if err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf("failed to decode JSON payload: %s", err), 400)
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
	if enreq.AttrReqs != nil {
		input.AttrReqs = []*mspApi.AttributeRequest{}
		for attr, optional := range enreq.AttrReqs {
			input.AttrReqs = append(input.AttrReqs, &mspApi.AttributeRequest{Name: attr, Optional: optional})
		}
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

func (w *idClientWrapper) Reenroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.IdentityResponse, *restutil.RestError) {
	username := params.ByName("username")
	enreq := identity.EnrollRequest{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&enreq)
	if err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf("failed to decode JSON payload: %s", err), 400)
	}

	input := mspApi.ReenrollmentRequest{
		Name:    username,
		CAName:  enreq.CAName,
		Profile: enreq.Profile,
	}
	if enreq.AttrReqs != nil {
		input.AttrReqs = []*mspApi.AttributeRequest{}
		for attr, optional := range enreq.AttrReqs {
			input.AttrReqs = append(input.AttrReqs, &mspApi.AttributeRequest{Name: attr, Optional: optional})
		}
	}

	err = w.caClient.Reenroll(&input)
	if err != nil {
		log.Errorf("Failed to re-enroll user %s. %s", enreq.Name, err)
		return nil, restutil.NewRestError(err.Error())
	}

	result := identity.IdentityResponse{
		Name:    enreq.Name,
		Success: true,
	}
	return &result, nil
}

func (w *idClientWrapper) Revoke(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RevokeResponse, *restutil.RestError) {
	username := params.ByName("username")
	enreq := identity.RevokeRequest{}
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&enreq)
	if err != nil {
		return nil, restutil.NewRestError(fmt.Sprintf("failed to decode JSON payload: %s", err), 400)
	}

	input := mspApi.RevocationRequest{
		Name:   username,
		CAName: enreq.CAName,
		Reason: enreq.Reason,
		GenCRL: enreq.GenCRL,
	}

	response, err := w.caClient.Revoke(&input)
	if err != nil {
		log.Errorf("Failed to revoke certificate for user %s. %s", enreq.Name, err)
		return nil, restutil.NewRestError(err.Error())
	}

	result := identity.RevokeResponse{
		CRL: string(response.CRL),
	}
	if len(response.RevokedCerts) > 0 {
		result.RevokedCerts = []map[string]string{}
		for _, cert := range response.RevokedCerts {
			entry := map[string]string{}
			entry["aki"] = cert.AKI
			entry["serial"] = cert.Serial
			result.RevokedCerts = append(result.RevokedCerts, entry)
		}
	}
	return &result, nil
}

func (w *idClientWrapper) List(res http.ResponseWriter, req *http.Request, params httprouter.Params) ([]*identity.Identity, *restutil.RestError) {
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
		if len(v.Attributes) > 0 {
			newId.Attributes = make(map[string]string, len(v.Attributes))
			for _, attr := range v.Attributes {
				newId.Attributes[attr.Name] = attr.Value
			}
		}
		ret[i] = &newId
	}
	return ret, nil
}

func (w *idClientWrapper) Get(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.Identity, *restutil.RestError) {
	username := params.ByName("username")
	result, err := w.caClient.GetIdentity(username, params.ByName("caname"))
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	newId := identity.Identity{}
	newId.Name = result.ID
	newId.MaxEnrollments = result.MaxEnrollments
	newId.CAName = result.CAName
	newId.Type = result.Type
	newId.Affiliation = result.Affiliation
	if len(result.Attributes) > 0 {
		newId.Attributes = make(map[string]string, len(result.Attributes))
		for _, attr := range result.Attributes {
			newId.Attributes[attr.Name] = attr.Value
		}
	}

	// the SDK identity manager does not persist the certificates
	// we have to retrieve it from the identity manager
	si, err := w.identityMgr.GetSigningIdentity(username)
	if err != nil && err != msp.ErrUserNotFound {
		return nil, restutil.NewRestError(err.Error(), 500)
	}
	if err == nil {
		// the user may have been enrolled by a different client instance
		ecert := si.EnrollmentCertificate()
		mspId := si.Identifier().MSPID
		newId.MSPID = mspId
		newId.EnrollmentCert = string(ecert)
	}
	newId.Organization = w.identityConfig.Client().Organization

	// the SDK doesn't save the CACert locally, we have to retrieve it from the Fabric CA server
	cacert, err := w.getCACert()
	if err != nil {
		return nil, restutil.NewRestError(err.Error(), 500)
	}

	newId.CACert = string(cacert)
	return &newId, nil
}

func (w *idClientWrapper) getCACert() ([]byte, error) {
	result, err := w.caClient.GetCAInfo()
	if err != nil {
		log.Errorf("Failed to retrieve Fabric CA information: %s", err)
		return nil, err
	}
	return result.CAChain, nil
}
