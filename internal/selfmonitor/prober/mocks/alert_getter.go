// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// AlertGetter is an autogenerated mock type for the alertGetter type
type AlertGetter struct {
	mock.Mock
}

// Alerts provides a mock function with given fields: ctx
func (_m *AlertGetter) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Alerts")
	}

	var r0 v1.AlertsResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.AlertsResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.AlertsResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.AlertsResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAlertGetter creates a new instance of AlertGetter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAlertGetter(t interface {
	mock.TestingT
	Cleanup(func())
}) *AlertGetter {
	mock := &AlertGetter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
