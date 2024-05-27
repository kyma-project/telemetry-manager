// Code generated by mockery v2.43.1. DO NOT EDIT.

package mocks

import (
	agent "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric/agent"

	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// AgentConfigBuilder is an autogenerated mock type for the AgentConfigBuilder type
type AgentConfigBuilder struct {
	mock.Mock
}

// Build provides a mock function with given fields: pipelines, options
func (_m *AgentConfigBuilder) Build(pipelines []v1alpha1.MetricPipeline, options agent.BuildOptions) *agent.Config {
	ret := _m.Called(pipelines, options)

	if len(ret) == 0 {
		panic("no return value specified for Build")
	}

	var r0 *agent.Config
	if rf, ok := ret.Get(0).(func([]v1alpha1.MetricPipeline, agent.BuildOptions) *agent.Config); ok {
		r0 = rf(pipelines, options)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*agent.Config)
		}
	}

	return r0
}

// NewAgentConfigBuilder creates a new instance of AgentConfigBuilder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAgentConfigBuilder(t interface {
	mock.TestingT
	Cleanup(func())
}) *AgentConfigBuilder {
	mock := &AgentConfigBuilder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
