// Code generated by mockery v2.40.2. DO NOT EDIT.

package mockasync

import (
	context "context"
	http "net/http"

	httprouter "github.com/julienschmidt/httprouter"

	messages "github.com/hyperledger/firefly-fabconnect/internal/messages"

	mock "github.com/stretchr/testify/mock"
)

// Dispatcher is an autogenerated mock type for the Dispatcher type
type Dispatcher struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Dispatcher) Close() {
	_m.Called()
}

// DispatchMsgAsync provides a mock function with given fields: ctx, msg, ack
func (_m *Dispatcher) DispatchMsgAsync(ctx context.Context, msg *messages.SendTransaction, ack bool) (*messages.AsyncSentMsg, error) {
	ret := _m.Called(ctx, msg, ack)

	if len(ret) == 0 {
		panic("no return value specified for DispatchMsgAsync")
	}

	var r0 *messages.AsyncSentMsg
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *messages.SendTransaction, bool) (*messages.AsyncSentMsg, error)); ok {
		return rf(ctx, msg, ack)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *messages.SendTransaction, bool) *messages.AsyncSentMsg); ok {
		r0 = rf(ctx, msg, ack)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*messages.AsyncSentMsg)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *messages.SendTransaction, bool) error); ok {
		r1 = rf(ctx, msg, ack)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// HandleReceipts provides a mock function with given fields: res, req, params
func (_m *Dispatcher) HandleReceipts(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	_m.Called(res, req, params)
}

// IsInitialized provides a mock function with given fields:
func (_m *Dispatcher) IsInitialized() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for IsInitialized")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Run provides a mock function with given fields:
func (_m *Dispatcher) Run() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Run")
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
func (_m *Dispatcher) ValidateConf() error {
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

// NewDispatcher creates a new instance of Dispatcher. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDispatcher(t interface {
	mock.TestingT
	Cleanup(func())
}) *Dispatcher {
	mock := &Dispatcher{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
