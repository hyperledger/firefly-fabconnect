// Code generated by mockery v1.0.0. DO NOT EDIT.

package mockidentity

import (
	http "net/http"

	identity "github.com/hyperledger/firefly-fabconnect/internal/rest/identity"
	httprouter "github.com/julienschmidt/httprouter"

	mock "github.com/stretchr/testify/mock"

	util "github.com/hyperledger/firefly-fabconnect/internal/rest/utils"
)

// IdentityClient is an autogenerated mock type for the IdentityClient type
type IdentityClient struct {
	mock.Mock
}

// Enroll provides a mock function with given fields: res, req, params
func (_m *IdentityClient) Enroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.IdentityResponse, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.IdentityResponse
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.IdentityResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.IdentityResponse)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

// Get provides a mock function with given fields: res, req, params
func (_m *IdentityClient) Get(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.Identity, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.Identity
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.Identity); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.Identity)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

// List provides a mock function with given fields: res, req, params
func (_m *IdentityClient) List(res http.ResponseWriter, req *http.Request, params httprouter.Params) ([]*identity.Identity, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 []*identity.Identity
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) []*identity.Identity); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*identity.Identity)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

// Modify provides a mock function with given fields: res, req, params
func (_m *IdentityClient) Modify(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RegisterResponse, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.RegisterResponse
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.RegisterResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.RegisterResponse)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

// Reenroll provides a mock function with given fields: res, req, params
func (_m *IdentityClient) Reenroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.IdentityResponse, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.IdentityResponse
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.IdentityResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.IdentityResponse)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

// Register provides a mock function with given fields: res, req, params
func (_m *IdentityClient) Register(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RegisterResponse, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.RegisterResponse
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.RegisterResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.RegisterResponse)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

// Revoke provides a mock function with given fields: res, req, params
func (_m *IdentityClient) Revoke(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.RevokeResponse, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.RevokeResponse
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.RevokeResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.RevokeResponse)
		}
	}

	var r1 *util.RestError
	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}
