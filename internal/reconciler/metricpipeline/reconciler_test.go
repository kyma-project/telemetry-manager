package metricpipeline

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
)

var (
	lockName = types.NamespacedName{
		Name:      "lock",
		Namespace: "default",
	}

	pipeline1 = telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-1",
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Output: telemetryv1alpha1.MetricPipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: "http://localhost:4317",
					},
				},
			},
		},
	}

	pipeline2 = telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipeline-2",
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Output: telemetryv1alpha1.MetricPipelineOutput{
				Otlp: &telemetryv1alpha1.OtlpOutput{
					Endpoint: telemetryv1alpha1.ValueType{
						Value: "http://localhost:4317",
					},
				},
			},
		},
	}

	pipelineWithSecretRef = telemetryv1alpha1.MetricPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelineWithSecretRef",
		},
		Spec: telemetryv1alpha1.MetricPipelineSpec{
			Output: telemetryv1alpha1.MetricPipelineOutput{
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

func TestGetDeployableMetricPipelines(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipeline1)
	require.NoError(t, err)

	pipelines := []telemetryv1alpha1.MetricPipeline{pipeline1}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, pipelines, fakeClient, l)
	require.NoError(t, err)
	require.Contains(t, deployablePipelines, pipeline1)
}

func TestMultipleGetDeployableMetricPipelines(t *testing.T) {
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

	pipelines := []telemetryv1alpha1.MetricPipeline{pipeline1, pipeline2}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, pipelines, fakeClient, l)
	require.NoError(t, err)
	require.Contains(t, deployablePipelines, pipeline1)
	require.Contains(t, deployablePipelines, pipeline2)
}

func TestMultipleGetDeployableMetricPipelinesWithoutLock(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipeline1)
	require.NoError(t, err)

	pipelines := []telemetryv1alpha1.MetricPipeline{pipeline1, pipeline2}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, pipelines, fakeClient, l)
	require.NoError(t, err)
	require.Contains(t, deployablePipelines, pipeline1)
	require.NotContains(t, deployablePipelines, pipeline2)
}

func TestGetDeployableMetricPipelinesWithMissingSecretReference(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipelineWithSecretRef)
	require.NoError(t, err)

	pipelines := []telemetryv1alpha1.MetricPipeline{pipelineWithSecretRef}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, pipelines, fakeClient, l)
	require.NoError(t, err)
	require.NotContains(t, deployablePipelines, pipelineWithSecretRef)
}

func TestGetDeployableMetricPipelinesWithoutLock(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	l := k8sutils.NewResourceCountLock(fakeClient, lockName, 2)

	err := l.TryAcquireLock(ctx, &pipelineWithSecretRef)
	require.NoError(t, err)

	pipelines := []telemetryv1alpha1.MetricPipeline{pipeline1}
	deployablePipelines, err := getDeployableMetricPipelines(ctx, pipelines, fakeClient, l)
	require.NoError(t, err)
	require.NotContains(t, deployablePipelines, pipeline1)
}
