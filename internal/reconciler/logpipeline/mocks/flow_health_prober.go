// Code generated by mockery v2.21.3. DO NOT EDIT.

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
func (_m *FlowHealthProber) Probe(ctx context.Context, pipelineName string) (prober.LogPipelineProbeResult, error) {
	ret := _m.Called(ctx, pipelineName)

	var r0 prober.LogPipelineProbeResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (prober.LogPipelineProbeResult, error)); ok {
		return rf(ctx, pipelineName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) prober.LogPipelineProbeResult); ok {
		r0 = rf(ctx, pipelineName)
	} else {
		r0 = ret.Get(0).(prober.LogPipelineProbeResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, pipelineName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewFlowHealthProber interface {
	mock.TestingT
	Cleanup(func())
}

// NewFlowHealthProber creates a new instance of FlowHealthProber. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewFlowHealthProber(t mockConstructorTestingTNewFlowHealthProber) *FlowHealthProber {
	mock := &FlowHealthProber{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
