// Code generated by mockery v2.43.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// IstioStatusChecker is an autogenerated mock type for the IstioStatusChecker type
type IstioStatusChecker struct {
	mock.Mock
}

// IsIstioActive provides a mock function with given fields: ctx
func (_m *IstioStatusChecker) IsIstioActive(ctx context.Context) bool {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for IsIstioActive")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context) bool); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// NewIstioStatusChecker creates a new instance of IstioStatusChecker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIstioStatusChecker(t interface {
	mock.TestingT
	Cleanup(func())
}) *IstioStatusChecker {
	mock := &IstioStatusChecker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}