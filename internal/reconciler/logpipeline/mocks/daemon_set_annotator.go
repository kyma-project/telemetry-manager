// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	types "k8s.io/apimachinery/pkg/types"
)

// DaemonSetAnnotator is an autogenerated mock type for the DaemonSetAnnotator type
type DaemonSetAnnotator struct {
	mock.Mock
}

// SetAnnotation provides a mock function with given fields: ctx, name, key, value
func (_m *DaemonSetAnnotator) SetAnnotation(ctx context.Context, name types.NamespacedName, key string, value string) error {
	ret := _m.Called(ctx, name, key, value)

	if len(ret) == 0 {
		panic("no return value specified for SetAnnotation")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, types.NamespacedName, string, string) error); ok {
		r0 = rf(ctx, name, key, value)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewDaemonSetAnnotator creates a new instance of DaemonSetAnnotator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDaemonSetAnnotator(t interface {
	mock.TestingT
	Cleanup(func())
}) *DaemonSetAnnotator {
	mock := &DaemonSetAnnotator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
