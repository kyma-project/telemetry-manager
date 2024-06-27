// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	prober "github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// FlowHealthProber is an autogenerated mock type for the FlowHealthProber type
type FlowHealthProber struct {
	mock.Mock
}

// Probe provides a mock function with given fields: ctx, pipelineName
func (_m *FlowHealthProber) Probe(ctx context.Context, pipelineName string) (prober.OTelPipelineProbeResult, error) {
	ret := _m.Called(ctx, pipelineName)

	if len(ret) == 0 {
		panic("no return value specified for Probe")
	}

	var r0 prober.OTelPipelineProbeResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (prober.OTelPipelineProbeResult, error)); ok {
		return rf(ctx, pipelineName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) prober.OTelPipelineProbeResult); ok {
		r0 = rf(ctx, pipelineName)
	} else {
		r0 = ret.Get(0).(prober.OTelPipelineProbeResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, pipelineName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewFlowHealthProber creates a new instance of FlowHealthProber. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewFlowHealthProber(t interface {
	mock.TestingT
	Cleanup(func())
}) *FlowHealthProber {
	mock := &FlowHealthProber{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
