package logpipeline

import (
	"context"
	"testing"

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
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("fluent bit is not ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							Value: "localhost",
						},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		agentHealthyCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionFalse, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonDaemonSetNotReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonDaemonSetNotReady, conditions.LogsMessage), agentHealthyCond.Message)
		require.Equal(t, updatedPipeline.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("fluent bit is ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							Value: "localhost",
						},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		agentHealthyCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionTrue, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonDaemonSetReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonDaemonSetReady, conditions.LogsMessage), agentHealthyCond.Message)
		require.Equal(t, updatedPipeline.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSReady, conditions.LogsMessage)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
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
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonReferencedSecretMissing, conditions.LogsMessage), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonReferencedSecretMissing, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("referenced secret exists", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
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
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionTrue, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonConfigurationGenerated, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonConfigurationGenerated, conditions.LogsMessage), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSReady, conditions.LogsMessage)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("loki output is defined", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
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
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonUnsupportedLokiOutput, conditions.LogsMessage), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonUnsupportedLokiOutput, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if fluent bit becomes not ready again", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypeAgentHealthy,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonDaemonSetReady,
						Message:            conditions.MessageFor(conditions.ReasonDaemonSetReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonConfigurationGenerated,
						Message:            conditions.MessageFor(conditions.ReasonConfigurationGenerated, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
				},
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
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if referenced secret does not exist anymore", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypeAgentHealthy,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonDaemonSetReady,
						Message:            conditions.MessageFor(conditions.ReasonDaemonSetReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonConfigurationGenerated,
						Message:            conditions.MessageFor(conditions.ReasonConfigurationGenerated, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
				},
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
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonReferencedSecretMissing, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if Loki output is defined", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypeAgentHealthy,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonDaemonSetReady,
						Message:            conditions.MessageFor(conditions.ReasonDaemonSetReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonConfigurationGenerated,
						Message:            conditions.MessageFor(conditions.ReasonConfigurationGenerated, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
				},
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
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonUnsupportedLokiOutput, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("should set status UnsupportedMode true if contains custom plugin", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				UnsupportedMode: false,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Filters: []telemetryv1alpha1.Filter{{Custom: "some-filter"}},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		require.True(t, updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				UnsupportedMode: true,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		require.False(t, updatedPipeline.Status.UnsupportedMode)
	})
}
