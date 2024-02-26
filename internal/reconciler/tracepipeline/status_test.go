package tracepipeline

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
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should add pending condition if trace gateway deployment is not ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentNotReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add running condition if trace gateway deployment is ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypeRunning, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if trace gateway deployment becomes not ready again", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
			Status: telemetryv1alpha1.TracePipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonTraceGatewayDeploymentNotReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonTraceGatewayDeploymentReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentReady),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentNotReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if some referenced secret does not exist anymore", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.TracePipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonTraceGatewayDeploymentNotReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonTraceGatewayDeploymentReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentReady),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
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

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonReferencedSecretMissing), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add running condition if referenced secret exists and trace gateway deployment is ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
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
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypeRunning, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add pending condition if waiting for lock", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, false)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonMaxPipelinesExceeded, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonMaxPipelinesExceeded), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add pending condition if acquired lock but trace gateway is not ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "localhost"},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DeploymentProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{Gateway: otelcollector.GatewayConfig{
				Config: otelcollector.Config{BaseName: "trace-gateway"},
			}},
			prober: proberStub,
		}
		err := sut.updateStatus(context.Background(), pipeline.Name, false)
		require.NoError(t, err)
		err = sut.updateStatus(context.Background(), pipeline.Name, true)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonTraceGatewayDeploymentNotReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})
}
