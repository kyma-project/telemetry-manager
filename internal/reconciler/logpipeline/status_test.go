package logpipeline

import (
	"context"
	"testing"
	"time"

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
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/tlscert"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("fluent bit is not ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

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
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		agentHealthyCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionFalse, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonAgentNotReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageForLogPipeline(conditions.ReasonAgentNotReady), agentHealthyCond.Message)
		require.Equal(t, updatedPipeline.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("fluent bit is ready", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

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
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		agentHealthyCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionTrue, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonAgentReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageForLogPipeline(conditions.ReasonAgentReady), agentHealthyCond.Message)
		require.Equal(t, updatedPipeline.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)

		conditionsSize := len(updatedPipeline.Status.Conditions)

		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-2]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionFalse, pendingCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)

		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("referenced secret missing", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

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
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForLogPipeline(conditions.ReasonReferencedSecretMissing), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonReferencedSecretMissing)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("referenced secret exists", func(t *testing.T) {
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithHTTPOutput(testutils.HTTPHostFromSecret("some-secret", "some-namespace", "host")).Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline, secret).WithStatusSubresource(&pipeline).Build()

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
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionTrue, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonConfigurationGenerated, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForLogPipeline(conditions.ReasonConfigurationGenerated), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		runningCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedPipeline.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("loki output is defined", func(t *testing.T) {
		pipeline := testutils.NewLogPipelineBuilder().WithName("pipeline").WithLokiOutput().Build()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

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
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

		configurationGeneratedCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		require.NotNil(t, configurationGeneratedCond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForLogPipeline(conditions.ReasonUnsupportedLokiOutput), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonUnsupportedLokiOutput)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("flow healthy", func(t *testing.T) {
		tests := []struct {
			name           string
			probe          prober.LogPipelineProbeResult
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
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{Healthy: true},
				},
				expectedStatus: metav1.ConditionTrue,
				expectedReason: conditions.ReasonFlowHealthy,
			},
			{
				name: "buffer filling up",
				probe: prober.LogPipelineProbeResult{
					BufferFillingUp: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonBufferFillingUp,
			},
			{
				name: "no logs delivered",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonNoLogsDelivered,
			},
			{
				name: "no logs delivered shadows other problems",
				probe: prober.LogPipelineProbeResult{
					NoLogsDelivered: true,
					BufferFillingUp: true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonNoLogsDelivered,
			},
			{
				name: "some data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSomeDataDropped,
			},
			{
				name: "some data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{SomeDataDropped: true},
					BufferFillingUp:     true,
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonSomeDataDropped,
			},
			{
				name: "all data dropped",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{AllDataDropped: true},
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonAllDataDropped,
			},
			{
				name: "all data dropped shadows other problems",
				probe: prober.LogPipelineProbeResult{
					PipelineProbeResult: prober.PipelineProbeResult{
						AllDataDropped:  true,
						SomeDataDropped: true,
					},
				},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonAllDataDropped,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				agentProberStub := &mocks.DaemonSetProber{}
				agentProberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

				flowHealthProberStub := &mocks.FlowHealthProber{}
				flowHealthProberStub.On("Probe", mock.Anything, pipeline.Name).Return(tt.probe, tt.probeErr)

				sut := Reconciler{
					Client:                   fakeClient,
					prober:                   agentProberStub,
					flowHealthProbingEnabled: true,
					flowHealthProber:         flowHealthProberStub,
				}
				err := sut.updateStatus(context.Background(), pipeline.Name)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)

				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeFlowHealthy)
				require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeFlowHealthy)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)
			})
		}
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
						Reason:             conditions.ReasonAgentReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonAgentReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonConfigurationGenerated,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonConfigurationGenerated),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady),
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
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady)
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
						Reason:             conditions.ReasonAgentReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonAgentReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonConfigurationGenerated,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonConfigurationGenerated),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady),
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
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonReferencedSecretMissing)
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
						Reason:             conditions.ReasonAgentReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonAgentReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeConfigurationGenerated,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonConfigurationGenerated,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonConfigurationGenerated),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.MessageForLogPipeline(conditions.ReasonFluentBitDSReady),
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
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonUnsupportedLokiOutput)
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

	t.Run("referenced secret does not exists for host and exists for tls cert", func(t *testing.T) {
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
						TLSConfig: telemetryv1alpha1.TLSConfig{
							Cert: &telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "cert",
								}},
							},
							Key: &telemetryv1alpha1.ValueType{
								ValueFrom: &telemetryv1alpha1.ValueFromSource{SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "key",
								}},
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
			Data: map[string][]byte{"cert": []byte("cert"), "key": []byte("key")},
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
		require.Equal(t, metav1.ConditionFalse, configurationGeneratedCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, configurationGeneratedCond.Reason)
		require.Equal(t, conditions.MessageForLogPipeline(conditions.ReasonReferencedSecretMissing), configurationGeneratedCond.Message)
		require.Equal(t, updatedPipeline.Generation, configurationGeneratedCond.ObservedGeneration)
		require.NotEmpty(t, configurationGeneratedCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedPipeline.Status.Conditions)
		pendingCond := updatedPipeline.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageForLogPipeline(conditions.ReasonReferencedSecretMissing)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedPipeline.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})
	t.Run("tls conditions", func(t *testing.T) {
		tests := []struct {
			name           string
			tlsCertErr     error
			expectedStatus metav1.ConditionStatus
			expectedReason string
		}{
			{
				name:           "cert expired",
				tlsCertErr:     &tlscert.CertExpiredError{Expiry: time.Now().Add(-time.Hour)},
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateExpired,
			},
			{
				name:           "cert about to expire",
				tlsCertErr:     &tlscert.CertAboutToExpireError{Expiry: time.Now().Add(7 * 24 * time.Hour)},
				expectedStatus: metav1.ConditionTrue,
				expectedReason: conditions.ReasonTLSCertificateAboutToExpire,
			},
			{
				name:           "cert decode failed",
				tlsCertErr:     tlscert.ErrCertDecodeFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
			{
				name:           "key decode failed",
				tlsCertErr:     tlscert.ErrKeyDecodeFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
			{
				name:           "key parse failed",
				tlsCertErr:     tlscert.ErrKeyParseFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
			{
				name:           "cert parse failed",
				tlsCertErr:     tlscert.ErrCertParseFailed,
				expectedStatus: metav1.ConditionFalse,
				expectedReason: conditions.ReasonTLSCertificateInvalid,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipeline := testutils.NewLogPipelineBuilder().WithHTTPOutput(testutils.HTTPClientTLS("", "fooCert", "fooKey")).Build()
				fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&pipeline).WithStatusSubresource(&pipeline).Build()

				proberStub := &mocks.DaemonSetProber{}
				proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
				tlsStub := &mocks.TLSCertValidator{}
				tlsStub.On("ValidateCertificate", mock.Anything, mock.Anything, mock.Anything).Return(tt.tlsCertErr)

				sut := Reconciler{
					Client:           fakeClient,
					tlsCertValidator: tlsStub,
					prober:           proberStub,
				}

				err := sut.updateStatus(context.Background(), pipeline.Name)
				require.NoError(t, err)

				var updatedPipeline telemetryv1alpha1.LogPipeline
				_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipeline.Name}, &updatedPipeline)
				cond := meta.FindStatusCondition(updatedPipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
				require.NotNil(t, cond, "could not find condition of type %s", conditions.TypeConfigurationGenerated)
				require.Equal(t, tt.expectedStatus, cond.Status)
				require.Equal(t, tt.expectedReason, cond.Reason)
			})
		}
	})

}
