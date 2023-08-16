// Code generated by mockery v2.32.0. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

// fileWriter is an autogenerated mock type for the fileWriter type
type fileWriter struct {
	mock.Mock
}

// prepareParserDryRun provides a mock function with given fields: ctx, workDir, pipeline
func (_m *fileWriter) prepareParserDryRun(ctx context.Context, workDir string, pipeline *v1alpha1.LogParser) (func(), error) {
	ret := _m.Called(ctx, workDir, pipeline)

	var r0 func()
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1alpha1.LogParser) (func(), error)); ok {
		return rf(ctx, workDir, pipeline)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1alpha1.LogParser) func()); ok {
		r0 = rf(ctx, workDir, pipeline)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func())
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *v1alpha1.LogParser) error); ok {
		r1 = rf(ctx, workDir, pipeline)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// preparePipelineDryRun provides a mock function with given fields: ctx, workDir, pipeline
func (_m *fileWriter) preparePipelineDryRun(ctx context.Context, workDir string, pipeline *v1alpha1.LogPipeline) (func(), error) {
	ret := _m.Called(ctx, workDir, pipeline)

	var r0 func()
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1alpha1.LogPipeline) (func(), error)); ok {
		return rf(ctx, workDir, pipeline)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *v1alpha1.LogPipeline) func()); ok {
		r0 = rf(ctx, workDir, pipeline)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func())
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *v1alpha1.LogPipeline) error); ok {
		r1 = rf(ctx, workDir, pipeline)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// newFileWriter creates a new instance of fileWriter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newFileWriter(t interface {
	mock.TestingT
	Cleanup(func())
}) *fileWriter {
	mock := &fileWriter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
