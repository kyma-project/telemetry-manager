// Code generated by mockery v2.32.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentHealthChecker is an autogenerated mock type for the ComponentHealthChecker type
type ComponentHealthChecker struct {
	mock.Mock
}

// check provides a mock function with given fields: ctx
func (_m *ComponentHealthChecker) check(ctx context.Context) (*v1.Condition, error) {
	ret := _m.Called(ctx)

	var r0 *v1.Condition
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*v1.Condition, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *v1.Condition); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.Condition)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewComponentHealthChecker creates a new instance of ComponentHealthChecker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewComponentHealthChecker(t interface {
	mock.TestingT
	Cleanup(func())
}) *ComponentHealthChecker {
	mock := &ComponentHealthChecker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
