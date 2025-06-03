package logpipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	logpipelinemocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestGetOutputType(t *testing.T) {
	type args struct {
		t *telemetryv1alpha1.LogPipeline
	}

	tests := []struct {
		name string
		args args
		want logpipelineutils.Mode
	}{
		{
			name: "OTel",
			args: args{
				&telemetryv1alpha1.LogPipeline{
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.LogPipelineOutput{
							OTLP: &telemetryv1alpha1.OTLPOutput{},
						},
					},
				},
			},

			want: logpipelineutils.OTel,
		},
		{
			name: "FluentBit",
			args: args{
				&telemetryv1alpha1.LogPipeline{
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.LogPipelineOutput{
							HTTP: &telemetryv1alpha1.LogPipelineHTTPOutput{},
						},
					},
				},
			},

			want: logpipelineutils.FluentBit,
		},
		{
			name: "OTel",
			args: args{
				&telemetryv1alpha1.LogPipeline{
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.LogPipelineOutput{
							Custom: "custom",
						},
					},
				},
			},

			want: logpipelineutils.FluentBit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := logpipelineutils.GetOutputType(tt.args.t); got != tt.want {
				t.Errorf("GetOutputType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPipelinesForType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	otelPipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	fluentbitPipeline1 := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()
	fluentbitPipeline2 := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&otelPipeline, &fluentbitPipeline1, &fluentbitPipeline2).WithStatusSubresource(&otelPipeline, &fluentbitPipeline1, &fluentbitPipeline2).Build()

	got, err := logpipelineutils.GetPipelinesForType(t.Context(), fakeClient, logpipelineutils.OTel)
	require.NoError(t, err)
	require.ElementsMatch(t, got, []telemetryv1alpha1.LogPipeline{otelPipeline})

	got, err = logpipelineutils.GetPipelinesForType(t.Context(), fakeClient, logpipelineutils.FluentBit)
	require.NoError(t, err)
	require.ElementsMatch(t, got, []telemetryv1alpha1.LogPipeline{fluentbitPipeline1, fluentbitPipeline2})
}

var _ LogPipelineReconciler = &ReconcilerStub{}

func TestRegisterAndCallRegisteredReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	otelPipeline := testutils.NewLogPipelineBuilder().WithOTLPOutput().Build()
	unsupportedPipeline := testutils.NewLogPipelineBuilder().WithHTTPOutput().Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&otelPipeline, &unsupportedPipeline).WithStatusSubresource(&otelPipeline, &unsupportedPipeline).Build()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", t.Context()).Return(&overrides.Config{}, nil)

	pipelineSync := &logpipelinemocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	otelReconciler := ReconcilerStub{
		OutputType: logpipelineutils.OTel,
		Result:     nil,
	}

	rec := New(fakeClient, overridesHandler, pipelineSync, &otelReconciler)

	res, err := rec.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: otelPipeline.Name},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	res, err = rec.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: unsupportedPipeline.Name},
	})
	require.ErrorIs(t, err, ErrUnsupportedOutputType)
	require.NotNil(t, res)
}

func TestReconcile_PausedOverride(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{
		Logging: overrides.LoggingConfig{Paused: true},
	}, nil)

	pipelineSync := &logpipelinemocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	rec := New(fakeClient, overridesHandler, pipelineSync)

	res, err := rec.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent-pipeline"},
	})
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, res)
}

func TestReconcile_MissingLogPipeline(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil)

	pipelineSync := &logpipelinemocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	rec := New(fakeClient, overridesHandler, pipelineSync)

	res, err := rec.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent-pipeline"},
	})
	require.NoError(t, err)
	require.Equal(t, ctrl.Result{}, res)
}

func TestReconcile_UnsupportedOutputType(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	unsupportedPipeline := testutils.NewLogPipelineBuilder().WithCustomOutput("custom").Build()

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&unsupportedPipeline).Build()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(&overrides.Config{}, nil)

	pipelineSync := &logpipelinemocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	rec := New(fakeClient, overridesHandler, pipelineSync)

	res, err := rec.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: unsupportedPipeline.Name},
	})
	require.ErrorIs(t, err, ErrUnsupportedOutputType)
	require.Equal(t, ctrl.Result{}, res)
}

func TestReconcile_LoadingOverridesFails(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	overridesHandler := &mocks.OverridesHandler{}
	overridesHandler.On("LoadOverrides", mock.Anything).Return(nil, fmt.Errorf("error loading overrides"))

	pipelineSync := &logpipelinemocks.PipelineSyncer{}
	pipelineSync.On("TryAcquireLock", mock.Anything, mock.Anything).Return(nil)

	rec := New(fakeClient, overridesHandler, pipelineSync)

	res, err := rec.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: "nonexistent-pipeline"},
	})
	require.Error(t, err)
	require.Equal(t, ctrl.Result{}, res)
}

// putting it here to avoid circular imports
var _ LogPipelineReconciler = &ReconcilerStub{}

type ReconcilerStub struct {
	OutputType logpipelineutils.Mode
	Result     error
}

func (r *ReconcilerStub) Reconcile(_ context.Context, _ *telemetryv1alpha1.LogPipeline) error {
	return r.Result
}

func (r *ReconcilerStub) SupportedOutput() logpipelineutils.Mode {
	return r.OutputType
}
