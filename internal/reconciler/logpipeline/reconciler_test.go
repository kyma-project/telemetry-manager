package logpipeline

import (
	"context"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

var (
	timestamp          = metav1.Now()
	pipelineInDeletion = telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pipelineInDeletion",
			DeletionTimestamp: &timestamp,
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				Custom: "Name	stdout\n",
			}},
	}

	pipelineWithSecret = telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelineWithSecret",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				HTTP: &telemetryv1alpha1.HTTPOutput{
					Host: telemetryv1alpha1.ValueType{
						ValueFrom: &telemetryv1alpha1.ValueFromSource{
							SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
								Name:      "some-secret",
								Namespace: "some-namespace",
								Key:       "host",
							},
						},
					},
				},
			}},
	}

	pipelineWithLokiOutput = telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelineWithLokiOutput",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				Loki: &telemetryv1alpha1.LokiOutput{
					URL: telemetryv1alpha1.ValueType{
						Value: "http://logging-loki:3100/loki/api/v1/push",
					},
				},
			}},
	}

	pipelineWithStdOut = telemetryv1alpha1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelineWithStdOut",
		},
		Spec: telemetryv1alpha1.LogPipelineSpec{
			Output: telemetryv1alpha1.Output{
				Custom: "Name	stdout\n",
			}},
	}
)

func TestGetDeployableLogPipelinesWithPipelineInDeletion(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	pipelines := []telemetryv1alpha1.LogPipeline{pipelineInDeletion}
	deployablePipelines := getDeployableLogPipelines(ctx, pipelines, fakeClient)
	require.NotContains(t, deployablePipelines, pipelineInDeletion)
}

func TestGetDeployableLogPipelinesWithMissingSecret(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	pipelines := []telemetryv1alpha1.LogPipeline{pipelineWithSecret}
	deployablePipelines := getDeployableLogPipelines(ctx, pipelines, fakeClient)
	require.NotContains(t, deployablePipelines, pipelineWithSecret)
}

func TestGetDeployableLogPipelinesWithLokiOutput(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	pipelines := []telemetryv1alpha1.LogPipeline{pipelineWithLokiOutput}
	deployablePipelines := getDeployableLogPipelines(ctx, pipelines, fakeClient)
	require.NotContains(t, deployablePipelines, pipelineWithLokiOutput)
}

func TestGetDeployableLogPipelinesWithStdOut(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	pipelines := []telemetryv1alpha1.LogPipeline{pipelineWithStdOut}
	deployablePipelines := getDeployableLogPipelines(ctx, pipelines, fakeClient)
	require.Contains(t, deployablePipelines, pipelineWithStdOut)
}
