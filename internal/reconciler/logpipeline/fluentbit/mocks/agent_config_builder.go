// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package mocks

import (
	"context"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	mock "github.com/stretchr/testify/mock"
)

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

// AgentConfigBuilder is an autogenerated mock type for the AgentConfigBuilder type
type AgentConfigBuilder struct {
	mock.Mock
}

type AgentConfigBuilder_Expecter struct {
	mock *mock.Mock
}

func (_m *AgentConfigBuilder) EXPECT() *AgentConfigBuilder_Expecter {
	return &AgentConfigBuilder_Expecter{mock: &_m.Mock}
}

// Build provides a mock function for the type AgentConfigBuilder
func (_mock *AgentConfigBuilder) Build(ctx context.Context, reconcilablePipelines []v1alpha1.LogPipeline) (*builder.FluentBitConfig, error) {
	ret := _mock.Called(ctx, reconcilablePipelines)

	if len(ret) == 0 {
		panic("no return value specified for Build")
	}

	var r0 *builder.FluentBitConfig
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, []v1alpha1.LogPipeline) (*builder.FluentBitConfig, error)); ok {
		return returnFunc(ctx, reconcilablePipelines)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, []v1alpha1.LogPipeline) *builder.FluentBitConfig); ok {
		r0 = returnFunc(ctx, reconcilablePipelines)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*builder.FluentBitConfig)
		}
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, []v1alpha1.LogPipeline) error); ok {
		r1 = returnFunc(ctx, reconcilablePipelines)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// AgentConfigBuilder_Build_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Build'
type AgentConfigBuilder_Build_Call struct {
	*mock.Call
}

// Build is a helper method to define mock.On call
//   - ctx context.Context
//   - reconcilablePipelines []v1alpha1.LogPipeline
func (_e *AgentConfigBuilder_Expecter) Build(ctx interface{}, reconcilablePipelines interface{}) *AgentConfigBuilder_Build_Call {
	return &AgentConfigBuilder_Build_Call{Call: _e.mock.On("Build", ctx, reconcilablePipelines)}
}

func (_c *AgentConfigBuilder_Build_Call) Run(run func(ctx context.Context, reconcilablePipelines []v1alpha1.LogPipeline)) *AgentConfigBuilder_Build_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 []v1alpha1.LogPipeline
		if args[1] != nil {
			arg1 = args[1].([]v1alpha1.LogPipeline)
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *AgentConfigBuilder_Build_Call) Return(fluentBitConfig *builder.FluentBitConfig, err error) *AgentConfigBuilder_Build_Call {
	_c.Call.Return(fluentBitConfig, err)
	return _c
}

func (_c *AgentConfigBuilder_Build_Call) RunAndReturn(run func(ctx context.Context, reconcilablePipelines []v1alpha1.LogPipeline) (*builder.FluentBitConfig, error)) *AgentConfigBuilder_Build_Call {
	_c.Call.Return(run)
	return _c
}
