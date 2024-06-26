// Code generated by mockery v2.40.2. DO NOT EDIT.

package mockreceiptapi

import mock "github.com/stretchr/testify/mock"

// ReceiptStorePersistence is an autogenerated mock type for the ReceiptStorePersistence type
type ReceiptStorePersistence struct {
	mock.Mock
}

// AddReceipt provides a mock function with given fields: requestID, receipt
func (_m *ReceiptStorePersistence) AddReceipt(requestID string, receipt *map[string]interface{}) error {
	ret := _m.Called(requestID, receipt)

	if len(ret) == 0 {
		panic("no return value specified for AddReceipt")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string, *map[string]interface{}) error); ok {
		r0 = rf(requestID, receipt)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Close provides a mock function with given fields:
func (_m *ReceiptStorePersistence) Close() {
	_m.Called()
}

// GetReceipt provides a mock function with given fields: requestID
func (_m *ReceiptStorePersistence) GetReceipt(requestID string) (*map[string]interface{}, error) {
	ret := _m.Called(requestID)

	if len(ret) == 0 {
		panic("no return value specified for GetReceipt")
	}

	var r0 *map[string]interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (*map[string]interface{}, error)); ok {
		return rf(requestID)
	}
	if rf, ok := ret.Get(0).(func(string) *map[string]interface{}); ok {
		r0 = rf(requestID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*map[string]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(requestID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetReceipts provides a mock function with given fields: skip, limit, ids, sinceEpochMS, from, to, start
func (_m *ReceiptStorePersistence) GetReceipts(skip int, limit int, ids []string, sinceEpochMS int64, from string, to string, start string) (*[]map[string]interface{}, error) {
	ret := _m.Called(skip, limit, ids, sinceEpochMS, from, to, start)

	if len(ret) == 0 {
		panic("no return value specified for GetReceipts")
	}

	var r0 *[]map[string]interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(int, int, []string, int64, string, string, string) (*[]map[string]interface{}, error)); ok {
		return rf(skip, limit, ids, sinceEpochMS, from, to, start)
	}
	if rf, ok := ret.Get(0).(func(int, int, []string, int64, string, string, string) *[]map[string]interface{}); ok {
		r0 = rf(skip, limit, ids, sinceEpochMS, from, to, start)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*[]map[string]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(int, int, []string, int64, string, string, string) error); ok {
		r1 = rf(skip, limit, ids, sinceEpochMS, from, to, start)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Init provides a mock function with given fields:
func (_m *ReceiptStorePersistence) Init() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Init")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ValidateConf provides a mock function with given fields:
func (_m *ReceiptStorePersistence) ValidateConf() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ValidateConf")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewReceiptStorePersistence creates a new instance of ReceiptStorePersistence. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewReceiptStorePersistence(t interface {
	mock.TestingT
	Cleanup(func())
}) *ReceiptStorePersistence {
	mock := &ReceiptStorePersistence{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
