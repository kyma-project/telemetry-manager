package metricpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	gatewayresources "github.com/kyma-project/telemetry-manager/internal/resources/otelcollector/gateway"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should add pending condition if metric gateway deployment is not ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonMetricGatewayDeploymentNotReady)
	})

	t.Run("should add running condition if metric gateway deployment is ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelineRunning)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonMetricGatewayDeploymentReady)
	})

	t.Run("should reset conditions and add pending condition if metric gateway deployment becomes not ready again", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
			Status: telemetryv1alpha1.MetricPipelineStatus{
				Conditions: []telemetryv1alpha1.MetricPipelineCondition{
					{Reason: reconciler.ReasonMetricGatewayDeploymentNotReady, Type: telemetryv1alpha1.MetricPipelinePending},
					{Reason: reconciler.ReasonMetricGatewayDeploymentReady, Type: telemetryv1alpha1.MetricPipelineRunning},
				},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonMetricGatewayDeploymentNotReady)
	})

	t.Run("should reset conditions and add pending condition if some referenced secret does not exist anymore", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{
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
			Status: telemetryv1alpha1.MetricPipelineStatus{
				Conditions: []telemetryv1alpha1.MetricPipelineCondition{
					{Reason: reconciler.ReasonMetricGatewayDeploymentNotReady, Type: telemetryv1alpha1.MetricPipelinePending},
					{Reason: reconciler.ReasonMetricGatewayDeploymentReady, Type: telemetryv1alpha1.MetricPipelineRunning},
				},
			},
		}

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonReferencedSecretMissingReason)
	})

	t.Run("should add running condition if referenced secret exists and metric gateway deployment is ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{
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
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelineRunning)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonMetricGatewayDeploymentReady)
	})

	t.Run("should add pending condition if waiting for lock", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, false)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonWaitingForLock)
	})

	t.Run("should add pending condition if acquired lock but metric gateway is not ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.MetricPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Spec: telemetryv1alpha1.MetricPipelineSpec{
				Output: telemetryv1alpha1.MetricPipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: gatewayresources.Config{BaseName: "metric-gateway"}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, false)
		require.NoError(t, err)
		err = sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.MetricPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Type, telemetryv1alpha1.MetricPipelinePending)
		require.Equal(t, updatedPipeline.Status.Conditions[0].Reason, reconciler.ReasonMetricGatewayDeploymentNotReady)
	})
}
