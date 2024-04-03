package tracepipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("trace gateway deployment is not ready", func(t *testing.T) {
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

		gatewayHealthyCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		require.NotNil(t, gatewayHealthyCond, "could not find condition of type %s", conditions.TypeGatewayHealthy)
		require.Equal(t, metav1.ConditionFalse, gatewayHealthyCond.Status)
		require.Equal(t, conditions.ReasonDeploymentNotReady, gatewayHealthyCond.Reason)
		require.Equal(t, conditions.MessageForTracePipeline(conditions.ReasonDeploymentNotReady), gatewayHealthyCond.Message)
		require.Equal(t, updatedPipeline.Generation, gatewayHealthyCond.ObservedGeneration)
		require.NotEmpty(t, gatewayHealthyCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("trace gateway deployment is ready", func(t *testing.T) {
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

		gatewayHealthyCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		require.NotNil(t, gatewayHealthyCond, "could not find condition of type %s", conditions.TypeGatewayHealthy)
		require.Equal(t, metav1.ConditionTrue, gatewayHealthyCond.Status)
		require.Equal(t, conditions.ReasonDeploymentReady, gatewayHealthyCond.Reason)
		require.Equal(t, conditions.MessageForTracePipeline(conditions.ReasonDeploymentReady), gatewayHealthyCond.Message)
		require.Equal(t, updatedPipeline.Generation, gatewayHealthyCond.ObservedGeneration)
		require.NotEmpty(t, gatewayHealthyCond.LastTransitionTime)

		conditionsSize := len(updatedPipeline.Status.Conditions)

		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-2]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionFalse, pendingCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)

		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
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
						Message:            conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonTraceGatewayDeploymentReady,
						Message:            conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady),
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

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForTracePipeline(conditions.ReasonReferencedSecretMissing), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonReferencedSecretMissing)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("referenced secret exists", func(t *testing.T) {
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

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionTrue, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonConfigurationGenerated, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForTracePipeline(conditions.ReasonConfigurationGenerated), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("waiting for lock", func(t *testing.T) {
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
		err := sut.updateStatus(context.Background(), pipeline.Name, false)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.TracePipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonMaxPipelinesExceeded, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonMaxPipelinesExceeded, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonMaxPipelinesExceeded)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name           string
			probe          prober.ProbeResult
			probeErr       error
			expectedStatus metav1.ConditionStatus
			expectedReason string
		}{
			{
				name:           "prober fails",
				probeErr:       assert.AnError,
				expectedStatus: metav1.ConditionUnknown,
				expectedReason: conditions.ReasonFlowHealthy,
			},
			{
				name: "healthy",
				probe: prober.ProbeResult{
					Healthy: true,
				},
				expectedStatus: metav1.ConditionTrue,
				expectedReason: conditions.ReasonFlowHealthy,
			},
			{
				name: "throttling",
				probe: prober.ProbeResult{
					Throttling: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonGatewayThrottling,
			},
			{
				name: "buffer filling up",
				probe: prober.ProbeResult{
					QueueAlmostFull: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonBufferFillingUp,
			},
			{
				name: "buffer filling up shadows other problems",
				probe: prober.ProbeResult{
					QueueAlmostFull: true,
					Throttling:      true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonBufferFillingUp,
			},
			{
				name: "some data dropped",
				probe: prober.ProbeResult{
					SomeDataDropped: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSomeDataDropped,
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.ProbeResult{
					SomeDataDropped: true,
					Throttling:      true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSomeDataDropped,
			},
			{
				name: "all data dropped",
				probe: prober.ProbeResult{
					AllDataDropped: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonAllDataDropped,
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.ProbeResult{
					AllDataDropped: true,
					Throttling:     true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonAllDataDropped,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewTracePipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				gatewayProberStub := &mocks.DeploymentProber{}
				gatewayProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				sut := Reconciler{
					Client:                   fakeClient,
					prober:                   gatewayProberStub,
					flowHealthProbingEnabled: true,
					flowHealthProber:         flowHealthProberStub,
				}
				err := sut.updateStatus(context.Background(), pipeline.Name, false)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.TracePipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeFlowHealthy)
				require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeFlowHealthy)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)
			})
		}
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
						Type:               conditions.TypeGatewayHealthy,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonDeploymentReady,
						Message:            conditions.MessageForTracePipeline(conditions.ReasonDeploymentReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.TypeConfigurationGenerated,
						Message:            conditions.MessageForTracePipeline(conditions.TypeConfigurationGenerated),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonTraceGatewayDeploymentNotReady,
						Message:            conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonTraceGatewayDeploymentReady,
						Message:            conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentReady),
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

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonTraceGatewayDeploymentNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForTracePipeline(conditions.ReasonTraceGatewayDeploymentNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})
}
