package logpipeline

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/stubs"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestGetOutputType(t *testing.T) {
	tests := []struct {
		name     string
		pipeline telemetryv1alpha1.LogPipeline
		want     logpipelineutils.Mode
	}{
		{
			name:     "OTLP output returns OTel mode",
			pipeline: testutils.NewLogPipelineBuilder().WithOTLPOutput().Build(),
			want:     logpipelineutils.OTel,
		},
		{
			name:     "HTTP output returns FluentBit mode",
			pipeline: testutils.NewLogPipelineBuilder().WithHTTPOutput().Build(),
			want:     logpipelineutils.FluentBit,
		},
		{
			name:     "Custom output returns FluentBit mode",
			pipeline: testutils.NewLogPipelineBuilder().WithCustomOutput("custom").Build(),
			want:     logpipelineutils.FluentBit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logpipelineutils.GetOutputType(&tt.pipeline)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGetPipelinesForType(t *testing.T) {
	otelPipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	fluentbitPipeline1 := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()
	fluentbitPipeline2 := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()

	fakeClient := newTestClient(t, &otelPipeline, &fluentbitPipeline1, &fluentbitPipeline2)

	got, err := logpipelineutils.GetPipelinesForType(t.Context(), fakeClient, logpipelineutils.OTel)
	require.NoError(t, err)
	require.ElementsMatch(t, got, []telemetryv1alpha1.LogPipeline{otelPipeline})

	got, err = logpipelineutils.GetPipelinesForType(t.Context(), fakeClient, logpipelineutils.FluentBit)
	require.NoError(t, err)
	require.ElementsMatch(t, got, []telemetryv1alpha1.LogPipeline{fluentbitPipeline1, fluentbitPipeline2})
}

var _ LogPipelineReconciler = &stubs.ReconcilerStub{}

func TestRegisterAndCallRegisteredReconciler(t *testing.T) {
	otelPipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	unsupportedPipeline := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()

	fakeClient := newTestClient(t, &otelPipeline, &unsupportedPipeline)

	otelReconciler := &stubs.ReconcilerStub{
		OutputType: logpipelineutils.OTel,
		Result:     nil,
	}

	rec := newTestReconciler(fakeClient, WithReconcilers(otelReconciler))

	result := reconcile(t, rec, otelPipeline.Name)
	require.NoError(t, result.err)
	require.NotNil(t, result.result)

	result = reconcile(t, rec, unsupportedPipeline.Name)
	require.ErrorIs(t, result.err, ErrUnsupportedOutputType)
	require.NotNil(t, result.result)
}

func TestReconcile_PausedOverride(t *testing.T) {
	fakeClient := newTestClient(t)

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{
		Logging: overrides.LoggingConfig{Paused: true},
	}, nil)

	rec := newTestReconciler(fakeClient, WithOverridesHandler(overridesHandler))

	result := reconcile(t, rec, "nonexistent-pipeline")
	require.NoError(t, result.err)
	require.Equal(t, ctrl.Result{}, result.result)
}

func TestReconcile_MissingLogPipeline(t *testing.T) {
	fakeClient := newTestClient(t)

	rec := newTestReconciler(fakeClient)

	result := reconcile(t, rec, "nonexistent-pipeline")
	require.NoError(t, result.err)
	require.Equal(t, ctrl.Result{}, result.result)
}

func TestReconcile_UnsupportedOutputType(t *testing.T) {
	unsupportedPipeline := testutils.NewLogPipelineBuilder().WithCustomOutput("custom").Build()

	fakeClient := newTestClient(t, &unsupportedPipeline)

	rec := newTestReconciler(fakeClient)

	result := reconcile(t, rec, unsupportedPipeline.Name)
	require.ErrorIs(t, result.err, ErrUnsupportedOutputType)
	require.Equal(t, ctrl.Result{}, result.result)
}

func TestReconcile_LoadingOverridesFails(t *testing.T) {
	fakeClient := newTestClient(t)

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(nil, fmt.Errorf("error loading overrides"))

	rec := newTestReconciler(fakeClient, WithOverridesHandler(overridesHandler))

	result := reconcile(t, rec, "nonexistent-pipeline")
	require.Error(t, result.err)
	require.Equal(t, ctrl.Result{}, result.result)
}
