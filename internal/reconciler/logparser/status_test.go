package logparser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logparser/mocks"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("fluent bit is not ready", func(t *testing.T) {
		parserName := "parser"
		parser := &telemetryv1alpha1.LogParser{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parserName,
				Generation: 1,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parser).WithStatusSubresource(parser).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{
				DaemonSet:        types.NamespacedName{Name: "fluent-bit"},
				ParsersConfigMap: types.NamespacedName{Name: "parsers"},
			},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), parser.Name)
		require.NoError(t, err)

		var updatedParser telemetryv1alpha1.LogParser
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: parserName}, &updatedParser)

		agentHealthyCond := meta.FindStatusCondition(updatedParser.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionFalse, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonDaemonSetNotReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonDaemonSetNotReady, conditions.LogsMessage), agentHealthyCond.Message)
		require.Equal(t, updatedParser.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)

		runningCond := meta.FindStatusCondition(updatedParser.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedParser.Status.Conditions)
		pendingCond := updatedParser.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedParser.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})

	t.Run("fluent bit is ready", func(t *testing.T) {
		parserName := "parser"
		parser := &telemetryv1alpha1.LogParser{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parserName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogParserStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parser).WithStatusSubresource(parser).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{
				DaemonSet:        types.NamespacedName{Name: "fluent-bit"},
				ParsersConfigMap: types.NamespacedName{Name: "parsers"},
			},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), parser.Name)
		require.NoError(t, err)

		var updatedParser telemetryv1alpha1.LogParser
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: parserName}, &updatedParser)

		agentHealthyCond := meta.FindStatusCondition(updatedParser.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionTrue, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonDaemonSetReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageFor(conditions.ReasonDaemonSetReady, conditions.LogsMessage), agentHealthyCond.Message)
		require.Equal(t, updatedParser.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)

		conditionsSize := len(updatedParser.Status.Conditions)
		runningCond := updatedParser.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypeRunning, runningCond.Type)
		require.Equal(t, metav1.ConditionTrue, runningCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, runningCond.Reason)
		runningCondMsg := conditions.RunningTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSReady, conditions.LogsMessage)
		require.Equal(t, runningCondMsg, runningCond.Message)
		require.Equal(t, updatedParser.Generation, runningCond.ObservedGeneration)
		require.NotEmpty(t, runningCond.LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if fluent bit becomes not ready again", func(t *testing.T) {
		parserName := "parser"
		parser := &telemetryv1alpha1.LogParser{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parserName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogParserStatus{
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
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parser).WithStatusSubresource(parser).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{
				DaemonSet:        types.NamespacedName{Name: "fluent-bit"},
				ParsersConfigMap: types.NamespacedName{Name: "parsers"},
			},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), parser.Name)
		require.NoError(t, err)

		var updatedParser telemetryv1alpha1.LogParser
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: parserName}, &updatedParser)

		runningCond := meta.FindStatusCondition(updatedParser.Status.Conditions, conditions.TypeRunning)
		require.Nil(t, runningCond)

		conditionsSize := len(updatedParser.Status.Conditions)
		pendingCond := updatedParser.Status.Conditions[conditionsSize-1]
		require.Equal(t, conditions.TypePending, pendingCond.Type)
		require.Equal(t, metav1.ConditionTrue, pendingCond.Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, pendingCond.Reason)
		pendingCondMsg := conditions.PendingTypeDeprecationMsg + conditions.MessageFor(conditions.ReasonFluentBitDSNotReady, conditions.LogsMessage)
		require.Equal(t, pendingCondMsg, pendingCond.Message)
		require.Equal(t, updatedParser.Generation, pendingCond.ObservedGeneration)
		require.NotEmpty(t, pendingCond.LastTransitionTime)
	})
}
