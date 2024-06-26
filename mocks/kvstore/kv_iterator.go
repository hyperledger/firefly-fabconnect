// Code generated by mockery v2.40.2. DO NOT EDIT.

package mockkvstore

import mock "github.com/stretchr/testify/mock"

// KVIterator is an autogenerated mock type for the KVIterator type
type KVIterator struct {
	mock.Mock
}

// Key provides a mock function with given fields:
func (_m *KVIterator) Key() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Key")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Last provides a mock function with given fields:
func (_m *KVIterator) Last() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Last")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Next provides a mock function with given fields:
func (_m *KVIterator) Next() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Next")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Prev provides a mock function with given fields:
func (_m *KVIterator) Prev() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Prev")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Release provides a mock function with given fields:
func (_m *KVIterator) Release() {
	_m.Called()
}

// Seek provides a mock function with given fields: _a0
func (_m *KVIterator) Seek(_a0 string) bool {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Seek")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(string) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Value provides a mock function with given fields:
func (_m *KVIterator) Value() []byte {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Value")
	}

	var r0 []byte
	if rf, ok := ret.Get(0).(func() []byte); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	return r0
}

// NewKVIterator creates a new instance of KVIterator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewKVIterator(t interface {
	mock.TestingT
	Cleanup(func())
}) *KVIterator {
	mock := &KVIterator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
