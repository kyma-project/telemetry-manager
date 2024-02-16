package logparser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	t.Run("should add pending condition if fluent bit is not ready", func(t *testing.T) {
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
		require.Len(t, updatedParser.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedParser.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedParser.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, updatedParser.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady), updatedParser.Status.Conditions[0].Message)
		require.Equal(t, updatedParser.Generation, updatedParser.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedParser.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add running condition if fluent bit becomes ready", func(t *testing.T) {
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
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady),
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
		require.Len(t, updatedParser.Status.Conditions, 2)

		require.Equal(t, conditions.TypePending, updatedParser.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionFalse, updatedParser.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, updatedParser.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady), updatedParser.Status.Conditions[0].Message)
		require.Equal(t, updatedParser.Generation, updatedParser.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedParser.Status.Conditions[0].LastTransitionTime)

		require.Equal(t, conditions.TypeRunning, updatedParser.Status.Conditions[1].Type)
		require.Equal(t, metav1.ConditionTrue, updatedParser.Status.Conditions[1].Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, updatedParser.Status.Conditions[1].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady), updatedParser.Status.Conditions[1].Message)
		require.Equal(t, updatedParser.Generation, updatedParser.Status.Conditions[1].ObservedGeneration)
		require.NotEmpty(t, updatedParser.Status.Conditions[1].LastTransitionTime)
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
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady),
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
		require.Len(t, updatedParser.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedParser.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedParser.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, updatedParser.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady), updatedParser.Status.Conditions[0].Message)
		require.Equal(t, updatedParser.Generation, updatedParser.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedParser.Status.Conditions[0].LastTransitionTime)
	})
}
