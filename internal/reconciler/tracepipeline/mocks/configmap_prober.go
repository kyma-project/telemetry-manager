// Code generated by mockery v2.16.0. DO NOT EDIT.

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

// IsPresent provides a mock function with given fields: ctx, name
func (_m *ConfigMapProber) IsPresent(ctx context.Context, name types.NamespacedName) (map[string]interface{}, error) {
	ret := _m.Called(ctx, name)

	var r0 map[string]interface{}
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName) map[string]interface{}); ok {
		r0 = rf(ctx, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]interface{})
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, types.NamespacedName) error); ok {
		r1 = rf(ctx, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewConfigMapProber interface {
	mock.TestingT
	Cleanup(func())
}

// NewConfigMapProber creates a new instance of ConfigMapProber. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewConfigMapProber(t mockConstructorTestingTNewConfigMapProber) *ConfigMapProber {
	mock := &ConfigMapProber{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
