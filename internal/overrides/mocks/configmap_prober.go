// Code generated by mockery v2.32.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	types "k8s.io/apimachinery/pkg/types"
)

// ConfigMapProber is an autogenerated mock type for the ConfigMapProber type
type ConfigMapProber struct {
	mock.Mock
}

// ReadConfigMapOrEmpty provides a mock function with given fields: ctx, name
func (_m *ConfigMapProber) ReadConfigMapOrEmpty(ctx context.Context, name types.NamespacedName) (string, error) {
	ret := _m.Called(ctx, name)

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName) (string, error)); ok {
		return rf(ctx, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName) string); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, types.NamespacedName) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewConfigMapProber creates a new instance of ConfigMapProber. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewConfigMapProber(t interface {
	mock.TestingT
	Cleanup(func())
}) *ConfigMapProber {
	mock := &ConfigMapProber{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
