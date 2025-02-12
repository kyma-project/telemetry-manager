package logparser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	commonStatusStubs "github.com/kyma-project/telemetry-manager/internal/reconciler/commonstatus/stubs"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
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

		proberStub := commonStatusStubs.NewDaemonSetProber(&workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "OOMKilled"})

		errToMsgConverter := &conditions.ErrorToMessageConverter{}

		sut := Reconciler{
			Client: fakeClient,
			config: Config{
				DaemonSet:        types.NamespacedName{Name: "fluent-bit"},
				ParsersConfigMap: types.NamespacedName{Name: "parsers"},
			},
			prober:         proberStub,
			errorConverter: errToMsgConverter,
		}

		err := sut.updateStatus(context.Background(), parser.Name)
		require.NoError(t, err)

		var updatedParser telemetryv1alpha1.LogParser
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: parserName}, &updatedParser)

		agentHealthyCond := meta.FindStatusCondition(updatedParser.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionFalse, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonAgentNotReady, agentHealthyCond.Reason)
		require.Equal(t, "Pod is in the pending state because container: foo is not running due to: OOMKilled. Please check the container: foo logs.", agentHealthyCond.Message)
		require.Equal(t, updatedParser.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)
	})

	t.Run("fluent bit is ready", func(t *testing.T) {
		parserName := "parser"
		parser := &telemetryv1alpha1.LogParser{
			ObjectMeta: metav1.ObjectMeta{
				Name:       parserName,
				Generation: 1,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parser).WithStatusSubresource(parser).Build()

		proberStub := commonStatusStubs.NewDaemonSetProber(nil)

		errToMsgConverter := &conditions.ErrorToMessageConverter{}

		sut := Reconciler{
			Client: fakeClient,
			config: Config{
				DaemonSet:        types.NamespacedName{Name: "fluent-bit"},
				ParsersConfigMap: types.NamespacedName{Name: "parsers"},
			},
			prober:         proberStub,
			errorConverter: errToMsgConverter,
		}

		err := sut.updateStatus(context.Background(), parser.Name)
		require.NoError(t, err)

		var updatedParser telemetryv1alpha1.LogParser
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: parserName}, &updatedParser)

		agentHealthyCond := meta.FindStatusCondition(updatedParser.Status.Conditions, conditions.TypeAgentHealthy)
		require.NotNil(t, agentHealthyCond, "could not find condition of type %s", conditions.TypeAgentHealthy)
		require.Equal(t, metav1.ConditionTrue, agentHealthyCond.Status)
		require.Equal(t, conditions.ReasonAgentReady, agentHealthyCond.Reason)
		require.Equal(t, conditions.MessageForFluentBitLogPipeline(conditions.ReasonAgentReady), agentHealthyCond.Message)
		require.Equal(t, updatedParser.Generation, agentHealthyCond.ObservedGeneration)
		require.NotEmpty(t, agentHealthyCond.LastTransitionTime)
	})
}
