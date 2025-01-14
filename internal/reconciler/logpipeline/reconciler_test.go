package logpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	pipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/pipelines"
)

func TestGetOutputType(t *testing.T) {
	type args struct {
		t *telemetryv1alpha1.LogPipeline
	}

	tests := []struct {
		name string
		args args
		want pipelineutils.LogPipelineMode
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

			want: pipelineutils.OTel,
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

			want: pipelineutils.FluentBit,
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

			want: pipelineutils.FluentBit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOutputType(tt.args.t); got != tt.want {
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

	got, err := GetPipelinesForType(context.Background(), fakeClient, pipelineutils.OTel)
	require.NoError(t, err)
	require.ElementsMatch(t, got, []telemetryv1alpha1.LogPipeline{otelPipeline})

	got, err = GetPipelinesForType(context.Background(), fakeClient, pipelineutils.FluentBit)
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
	overridesHandler.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	otelReconciler := ReconcilerStub{
		OutputType: pipelineutils.OTel,
		Result:     nil,
	}

	rec := New(fakeClient, overridesHandler, &otelReconciler)

	res, err := rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: otelPipeline.Name},
	})
	require.NoError(t, err)
	require.NotNil(t, res)

	res, err = rec.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Name: unsupportedPipeline.Name},
	})
	require.ErrorIs(t, err, ErrUnsupportedOutputType)
	require.NotNil(t, res)
}

// putting it here to avoid circular imports
var _ LogPipelineReconciler = &ReconcilerStub{}

type ReconcilerStub struct {
	OutputType pipelineutils.LogPipelineMode
	Result     error
}

func (r *ReconcilerStub) Reconcile(_ context.Context, _ *telemetryv1alpha1.LogPipeline) error {
	return r.Result
}

func (r *ReconcilerStub) SupportedOutput() pipelineutils.LogPipelineMode {
	return r.OutputType
}
