// Code generated by mockery v2.43.1. DO NOT EDIT.

package mocks

import (
	context "context"

	gateway "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/gateway"

	mock "github.com/stretchr/testify/mock"

	otlpexporter "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"

	v1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// GatewayConfigBuilder is an autogenerated mock type for the GatewayConfigBuilder type
type GatewayConfigBuilder struct {
	mock.Mock
}

// Build provides a mock function with given fields: ctx, pipelines
func (_m *GatewayConfigBuilder) Build(ctx context.Context, pipelines []v1alpha1.MetricPipeline) (*gateway.Config, otlpexporter.EnvVars, error) {
	ret := _m.Called(ctx, pipelines)

	if len(ret) == 0 {
		panic("no return value specified for Build")
	}

	var r0 *gateway.Config
	var r1 otlpexporter.EnvVars
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []v1alpha1.MetricPipeline) (*gateway.Config, otlpexporter.EnvVars, error)); ok {
		return rf(ctx, pipelines)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []v1alpha1.MetricPipeline) *gateway.Config); ok {
		r0 = rf(ctx, pipelines)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*gateway.Config)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []v1alpha1.MetricPipeline) otlpexporter.EnvVars); ok {
		r1 = rf(ctx, pipelines)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(otlpexporter.EnvVars)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, []v1alpha1.MetricPipeline) error); ok {
		r2 = rf(ctx, pipelines)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// NewGatewayConfigBuilder creates a new instance of GatewayConfigBuilder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewGatewayConfigBuilder(t interface {
	mock.TestingT
	Cleanup(func())
}) *GatewayConfigBuilder {
	mock := &GatewayConfigBuilder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
