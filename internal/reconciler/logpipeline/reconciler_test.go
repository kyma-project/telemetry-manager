package logpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestGetDeployableLogPipelines(t *testing.T) {
	timestamp := metav1.Now()
	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.LogPipeline
		deployablePipelines bool
	}{
		{
			name: "should reject LogPipelines which are being deleted",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "pipeline-in-deletion",
						DeletionTimestamp: &timestamp,
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "Name	stdout\n",
						}},
				},
			},
			deployablePipelines: false,
		},
		{
			name: "should reject LogPipelines with missing Secrets",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-secret",
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
				},
			},
			deployablePipelines: false,
		},
		{
			name: "should reject LogPipelines with Loki Output",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-loki-output",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Loki: &telemetryv1alpha1.LokiOutput{
								URL: telemetryv1alpha1.ValueType{
									Value: "http://logging-loki:3100/loki/api/v1/push",
								},
							},
						}},
				},
			},
			deployablePipelines: false,
		},
		{
			name: "should accept healthy LogPipelines",
			pipelines: []telemetryv1alpha1.LogPipeline{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-stdout-1",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "Name	stdout\n",
						}},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pipeline-with-stdout-2",
					},
					Spec: telemetryv1alpha1.LogPipelineSpec{
						Output: telemetryv1alpha1.Output{
							Custom: "Name	stdout\n",
						}},
				},
			},
			deployablePipelines: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

			deployablePipelines := getDeployableLogPipelines(ctx, test.pipelines, fakeClient)
			for _, pipeline := range test.pipelines {
				if test.deployablePipelines == true {
					require.Contains(t, deployablePipelines, pipeline)
				} else {
					require.NotContains(t, deployablePipelines, pipeline)
				}
			}
		})
	}
}
