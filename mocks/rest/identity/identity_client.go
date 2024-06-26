// Code generated by mockery v2.29.0. DO NOT EDIT.

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
func (_m *IdentityClient) Enroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.Response, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.Response
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) (*identity.Response, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.Response); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.Response)
		}
	}

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

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 *identity.Identity
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) (*identity.Identity, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.Identity); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.Identity)
		}
	}

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

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 []*identity.Identity
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) ([]*identity.Identity, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) []*identity.Identity); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*identity.Identity)
		}
	}

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

	if len(ret) == 0 {
		panic("no return value specified for Modify")
	}

	var r0 *identity.RegisterResponse
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) (*identity.RegisterResponse, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.RegisterResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.RegisterResponse)
		}
	}

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
func (_m *IdentityClient) Reenroll(res http.ResponseWriter, req *http.Request, params httprouter.Params) (*identity.Response, *util.RestError) {
	ret := _m.Called(res, req, params)

	var r0 *identity.Response
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) (*identity.Response, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.Response); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.Response)
		}
	}

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

	if len(ret) == 0 {
		panic("no return value specified for Register")
	}

	var r0 *identity.RegisterResponse
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) (*identity.RegisterResponse, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.RegisterResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.RegisterResponse)
		}
	}

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

	if len(ret) == 0 {
		panic("no return value specified for Revoke")
	}

	var r0 *identity.RevokeResponse
	var r1 *util.RestError
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) (*identity.RevokeResponse, *util.RestError)); ok {
		return rf(res, req, params)
	}
	if rf, ok := ret.Get(0).(func(http.ResponseWriter, *http.Request, httprouter.Params) *identity.RevokeResponse); ok {
		r0 = rf(res, req, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*identity.RevokeResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(http.ResponseWriter, *http.Request, httprouter.Params) *util.RestError); ok {
		r1 = rf(res, req, params)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*util.RestError)
		}
	}

	return r0, r1
}

type mockConstructorTestingTNewIdentityClient interface {
	mock.TestingT
	Cleanup(func())
}

// NewIdentityClient creates a new instance of IdentityClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewIdentityClient(t mockConstructorTestingTNewIdentityClient) *IdentityClient {
	mock := &IdentityClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
