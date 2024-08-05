package commonstatus

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	logMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
	metricMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/metricpipeline/mocks"
	traceMocks "github.com/kyma-project/telemetry-manager/internal/reconciler/tracepipeline/mocks"
	"github.com/kyma-project/telemetry-manager/internal/workloadstatus"
)

func TestTracesGetHealthCondition(t *testing.T) {
	tests := []struct {
		name              string
		proberErr         error
		expectedCondition *metav1.Condition
	}{
		{
			name:      "Test GetHealthCondition with signal type traces",
			proberErr: nil,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonGatewayReady,
				Message: conditions.MessageForTracePipeline(conditions.ReasonGatewayReady),
			},
		},
		{
			name:      "Test GetHealthCondition with signal type traces and error",
			proberErr: workloadstatus.ErrDeploymentFetching,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonGatewayNotReady,
				Message: conditions.ConvertErrToMsg(workloadstatus.ErrDeploymentFetching),
			},
		},
		{
			name:      "Test GetHealthCondition with signal type traces and rollout in progress error",
			proberErr: &workloadstatus.RolloutInProgressError{},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonRolloutInProgress,
				Message: "Pods are being started/updated",
			},
		},
		{
			name:      "Test GetHealthCondition with signal type traces and wrapped error",
			proberErr: fmt.Errorf("new error: %w", &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "foo"}),
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonGatewayNotReady,
				Message: "Pod is in the pending state as container: foo is not running due to: foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gatewayProberStub := &traceMocks.DeploymentProber{}
			gatewayProberStub.On("IsReady", context.TODO(), types.NamespacedName{}).Return(tt.proberErr)
			actualCondition := GetGatewayHealthyCondition(context.TODO(), gatewayProberStub, types.NamespacedName{}, &conditions.ErrorToMessageConverter{}, SignalTypeTraces)
			require.True(t, validateCondition(t, tt.expectedCondition, actualCondition))
		})
	}
}

func TestMetricsGetHealthCondition(t *testing.T) {
	tests := []struct {
		name                     string
		proberAgentErr           error
		preberGatewayErr         error
		expectedAgentCondition   *metav1.Condition
		expectedGatewayCondition *metav1.Condition
	}{
		{
			name:             "Test GetHealthCondition with signal type metrics",
			proberAgentErr:   nil,
			preberGatewayErr: nil,
			expectedGatewayCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonGatewayReady,
				Message: conditions.MessageForMetricPipeline(conditions.ReasonGatewayReady),
			},
			expectedAgentCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonAgentReady,
				Message: conditions.MessageForMetricPipeline(conditions.ReasonAgentReady),
			},
		},
		{
			name:             "Test GetHealthCondition with signal type metrics and error",
			proberAgentErr:   workloadstatus.ErrDaemonSetNotFound,
			preberGatewayErr: workloadstatus.ErrDeploymentFetching,
			expectedGatewayCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonGatewayNotReady,
				Message: conditions.ConvertErrToMsg(workloadstatus.ErrDeploymentFetching),
			},
			expectedAgentCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonAgentNotReady,
				Message: conditions.ConvertErrToMsg(workloadstatus.ErrDaemonSetNotFound),
			},
		},
		{
			name:             "Test GetHealthCondition with signal type metrics and rollout in progress error",
			proberAgentErr:   &workloadstatus.RolloutInProgressError{},
			preberGatewayErr: &workloadstatus.RolloutInProgressError{},
			expectedGatewayCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonRolloutInProgress,
				Message: "Pods are being started/updated",
			},
			expectedAgentCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonRolloutInProgress,
				Message: "Pods are being started/updated",
			},
		},
		{
			name:             "Test GetHealthCondition with signal type metrics and wrapped error",
			proberAgentErr:   fmt.Errorf("new error: %w", &workloadstatus.PodIsFailingError{Message: "foo"}),
			preberGatewayErr: fmt.Errorf("new error: %w", &workloadstatus.PodIsPendingError{ContainerName: "foo", Message: "fooMessage"}),
			expectedGatewayCondition: &metav1.Condition{
				Type:    conditions.TypeGatewayHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonGatewayNotReady,
				Message: "Pod is in the pending state as container: foo is not running due to: fooMessage",
			},
			expectedAgentCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonAgentNotReady,
				Message: "Pod is in the failed state due to: foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			agentProberStub := &metricMocks.DaemonSetProber{}
			agentProberStub.On("IsReady", context.TODO(), types.NamespacedName{}).Return(tt.proberAgentErr)
			gatewayProberStub := &metricMocks.DeploymentProber{}
			gatewayProberStub.On("IsReady", context.TODO(), types.NamespacedName{}).Return(tt.preberGatewayErr)

			actualAgentCondition := GetAgentHealthyCondition(context.TODO(), agentProberStub, types.NamespacedName{}, &conditions.ErrorToMessageConverter{}, SignalTypeMetrics)
			actualGatewayCondition := GetGatewayHealthyCondition(context.TODO(), gatewayProberStub, types.NamespacedName{}, &conditions.ErrorToMessageConverter{}, SignalTypeMetrics)

			require.True(t, validateCondition(t, tt.expectedAgentCondition, actualAgentCondition))
			require.True(t, validateCondition(t, tt.expectedGatewayCondition, actualGatewayCondition))
		})
	}
}

func TestLogsGetHealthCondition(t *testing.T) {
	tests := []struct {
		name              string
		proberErr         error
		expectedCondition *metav1.Condition
	}{
		{
			name:      "Test GetHealthCondition with signal type logs",
			proberErr: nil,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonAgentReady,
				Message: conditions.MessageForLogPipeline(conditions.ReasonAgentReady),
			},
		},
		{
			name:      "Test GetHealthCondition with signal type logs and error",
			proberErr: workloadstatus.ErrDaemonSetNotFound,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonAgentNotReady,
				Message: conditions.ConvertErrToMsg(workloadstatus.ErrDaemonSetNotFound),
			},
		},
		{
			name:      "Test GetHealthCondition with signal type logs and rollout in progress error",
			proberErr: &workloadstatus.RolloutInProgressError{},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionTrue,
				Reason:  conditions.ReasonRolloutInProgress,
				Message: "Pods are being started/updated",
			},
		},
		{
			name:      "Test GetHealthCondition with signal type logs and wrapped error",
			proberErr: fmt.Errorf("new error: %w", &workloadstatus.PodIsNotScheduledError{Message: "foo"}),
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeAgentHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  conditions.ReasonAgentNotReady,
				Message: "Pod is not scheduled: foo",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentProberStub := &logMocks.DaemonSetProber{}
			agentProberStub.On("IsReady", context.TODO(), types.NamespacedName{}).Return(tt.proberErr)
			actualCondition := GetAgentHealthyCondition(context.TODO(), agentProberStub, types.NamespacedName{}, &conditions.ErrorToMessageConverter{}, SignalTypeLogs)
			require.True(t, validateCondition(t, tt.expectedCondition, actualCondition))
		})
	}
}

func validateCondition(t *testing.T, exected, actual *metav1.Condition) bool {
	require.Equal(t, exected.Type, actual.Type)
	require.Equal(t, exected.Status, actual.Status)
	require.Equal(t, exected.Reason, actual.Reason)
	require.Equal(t, exected.Message, actual.Message)
	return true
}
