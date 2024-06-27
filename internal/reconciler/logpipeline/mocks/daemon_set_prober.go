// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	types "k8s.io/apimachinery/pkg/types"
)

// DaemonSetProber is an autogenerated mock type for the DaemonSetProber type
type DaemonSetProber struct {
	mock.Mock
}

// IsReady provides a mock function with given fields: ctx, name
func (_m *DaemonSetProber) IsReady(ctx context.Context, name types.NamespacedName) (bool, error) {
	ret := _m.Called(ctx, name)

	if len(ret) == 0 {
		panic("no return value specified for IsReady")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName) (bool, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName) bool); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.NamespacedName) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewDaemonSetProber creates a new instance of DaemonSetProber. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDaemonSetProber(t interface {
	mock.TestingT
	Cleanup(func())
}) *DaemonSetProber {
	mock := &DaemonSetProber{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
