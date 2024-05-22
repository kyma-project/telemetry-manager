package tracepipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	lockName = types.NamespacedName{
		Name:      "lock",
		Namespace: "default",
	}

	pipeline1 = telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: "http://localhost:4317",
					},
				},
			},
		},
	}

	pipeline2 = telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-2",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: "http://localhost:4317",
					},
				},
			},
		},
	}

	pipelineWithSecretRef = telemetryv1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelineWithSecretRef",
		},
		Spec: telemetryv1alpha1.TracePipelineSpec{
			Output: telemetryv1alpha1.TracePipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Key:       "key",
								Name:      "secret",
								Namespace: "default",
							},
						},
					},
				},
			},
		},
	}
)

func TestReconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().
		WithObjects(ptr.To(testutils.NewTracePipelineBuilder().WithName("test").Build())).
		WithScheme(scheme).
		Build()

	overridesHandlerStub := &mocks.OverridesHandler{}
	overridesHandlerStub.On("LoadOverrides", context.Background()).Return(&overrides.Config{}, nil)

	gatewayProberStub := &mocks.DeploymentProber{}
	gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

	flowHealthProberStub := &mocks.FlowHealthProber{}
	flowHealthProberStub.On("Probe", mock.Anything, mock.Anything).Return(prober.OTelPipelineProbeResult{}, nil)

	tlsCertValidatorStub := &mocks.TLSCertValidator{}
	tlsCertValidatorStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	istioStatusCheckerStub := &mocks.IstioStatusChecker{}
	istioStatusCheckerStub.On("IsIstioActive", mock.Anything).Return(false)

	sut := Reconciler{
		Client: fakeClient,
		config: Config{
			Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{
					BaseName:  "gateway",
					Namespace: "default",
				},
				Deployment: otelcollector.DeploymentConfig{
					Image: "otel/opentelemetry-collector-contrib",
				},
				OTLPServiceName: "otlp",
			},
			MaxPipelines: 3,
		},
		pipelinesConditionsCleared: true,
		prober:                     gatewayProberStub,
		flowHealthProbingEnabled:   false,
		flowHealthProber:           flowHealthProberStub,
		tlsCertValidator:           tlsCertValidatorStub,
		overridesHandler:           overridesHandlerStub,
		istioStatusChecker:         istioStatusCheckerStub,
	}

	_, err := sut.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "test",
		},
	})
	require.NoError(t, err)
}

func TestGetDeployableTracePipelines(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipeline1)
	require.NoError(t, err)

	validatorStub := &mocks.TLSCertValidator{}
	pipelines := []telemetryv1alpha1.TracePipeline{pipeline1}
	reconciler := Reconciler{
		Client:           fakeClient,
		tlsCertValidator: validatorStub,
	}
	deployablePipelines, err := reconciler.getReconcilablePipelines(ctx, pipelines, l)
	require.NoError(t, err)
	require.Contains(t, deployablePipelines, pipeline1)
}

func TestMultipleGetDeployableTracePipelines(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipeline1)
	require.NoError(t, err)

	err = l.TryAcquireLock(ctx, &pipeline2)
	require.NoError(t, err)

	validatorStub := &mocks.TLSCertValidator{}
	pipelines := []telemetryv1alpha1.TracePipeline{pipeline1, pipeline2}
	reconciler := Reconciler{
		Client:           fakeClient,
		tlsCertValidator: validatorStub,
	}
	deployablePipelines, err := reconciler.getReconcilablePipelines(ctx, pipelines, l)
	require.NoError(t, err)
	require.Contains(t, deployablePipelines, pipeline1)
	require.Contains(t, deployablePipelines, pipeline2)
}

func TestMultipleGetDeployableTracePipelinesWithoutLock(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipeline1)
	require.NoError(t, err)

	validatorStub := &mocks.TLSCertValidator{}
	pipelines := []telemetryv1alpha1.TracePipeline{pipeline1, pipeline2}
	reconciler := Reconciler{
		Client:           fakeClient,
		tlsCertValidator: validatorStub,
	}
	deployablePipelines, err := reconciler.getReconcilablePipelines(ctx, pipelines, l)
	require.NoError(t, err)
	require.Contains(t, deployablePipelines, pipeline1)
	require.NotContains(t, deployablePipelines, pipeline2)
}

func TestGetDeployableTracePipelinesWithMissingSecretReference(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipelineWithSecretRef)
	require.NoError(t, err)

	validatorStub := &mocks.TLSCertValidator{}
	pipelines := []telemetryv1alpha1.TracePipeline{pipelineWithSecretRef}
	reconciler := Reconciler{
		Client:           fakeClient,
		tlsCertValidator: validatorStub,
	}
	deployablePipelines, err := reconciler.getReconcilablePipelines(ctx, pipelines, l)
	require.NoError(t, err)
	require.NotContains(t, deployablePipelines, pipelineWithSecretRef)
}

func TestGetDeployableTracePipelinesWithoutLock(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipelineWithSecretRef)
	require.NoError(t, err)

	validatorStub := &mocks.TLSCertValidator{}
	pipelines := []telemetryv1alpha1.TracePipeline{pipeline1}
	reconciler := Reconciler{
		Client:           fakeClient,
		tlsCertValidator: validatorStub,
	}
	deployablePipelines, err := reconciler.getReconcilablePipelines(ctx, pipelines, l)
	require.NoError(t, err)
	require.NotContains(t, deployablePipelines, pipeline1)
}
