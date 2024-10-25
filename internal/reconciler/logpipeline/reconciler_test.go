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
)

func TestGetOutputType(t *testing.T) {
	type args struct {
		t *telemetryv1alpha1.LogPipeline
	}

	tests := []struct {
		name string
		args args
		want telemetryv1alpha1.Mode
	}{
		{
			name: "OTel",
			args: args{
				&telemetryv1alpha1.LogPipeline{
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							OTLP: &telemetryv1alpha1.OTLPOutput{},
						},
					},
				},
			},

			want: telemetryv1alpha1.OTel,
		},
		{
			name: "FluentBit",
			args: args{
				&telemetryv1alpha1.LogPipeline{
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							HTTP: &telemetryv1alpha1.HTTPOutput{},
						},
					},
				},
			},

			want: telemetryv1alpha1.FluentBit,
		},
		{
			name: "OTel",
			args: args{
				&telemetryv1alpha1.LogPipeline{
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "custom",
						},
					},
				},
			},

			want: telemetryv1alpha1.FluentBit,
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

	got, err := GetPipelinesForType(context.Background(), fakeClient, telemetryv1alpha1.OTel)
	require.NoError(t, err)
	require.ElementsMatch(t, got, []telemetryv1alpha1.LogPipeline{otelPipeline})

	got, err = GetPipelinesForType(context.Background(), fakeClient, telemetryv1alpha1.FluentBit)
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
		OutputType: telemetryv1alpha1.OTel,
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
	OutputType telemetryv1alpha1.Mode
	Result     error
}

func (r *ReconcilerStub) Reconcile(_ context.Context, _ *telemetryv1alpha1.LogPipeline) error {
	return r.Result
}

func (r *ReconcilerStub) SupportedOutput() telemetryv1alpha1.Mode {
	return r.OutputType
}
