package telemetry

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUpdateStatus(t *testing.T) {
	tests := []struct {
		name                 string
		config               *Config
		telemetry            *operatorv1alpha1.Telemetry
		resources            []client.Object
		logsCheckerReturn    *metav1.Condition
		logsCheckerError     error
		metricsCheckerReturn *metav1.Condition
		metricsCheckerError  error
		tracesCheckerReturn  *metav1.Condition
		tracesCheckerError   error
		expectedState        operatorv1alpha1.State
		expectedConditions   []metav1.Condition
		expectedEndpoints    operatorv1alpha1.GatewayEndpoints
		expectError          bool
	}{
		{
			name: "all components are healthy",
			config: &Config{
				Traces:  TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
				Metrics: MetricsConfig{Enabled: true},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateReady,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{Traces: &operatorv1alpha1.OTLPEndpoints{
				GRPC: "http://traces.telemetry-system:4317",
				HTTP: "http://traces.telemetry-system:4318",
			}},
		},
		{
			name: "non trace components are unhealthy",
			config: &Config{
				Traces:  TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
				Metrics: MetricsConfig{Enabled: true},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionFalse, Reason: reconciler.ReasonFluentBitDSNotReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionFalse, Reason: reconciler.ReasonFluentBitDSNotReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{Traces: &operatorv1alpha1.OTLPEndpoints{
				GRPC: "http://traces.telemetry-system:4317",
				HTTP: "http://traces.telemetry-system:4318",
			}},
		},
		{
			name: "trace components are unhealthy",
			config: &Config{
				Metrics: MetricsConfig{Enabled: true},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionFalse, Reason: reconciler.ReasonTraceGatewayDeploymentNotReady},
			expectedState:        operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionFalse, Reason: reconciler.ReasonTraceGatewayDeploymentNotReady},
			},
		},
		{
			name: "metrics are unhealthy but not enabled",
			config: &Config{
				Traces:  TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
				Metrics: MetricsConfig{Enabled: false},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateReady,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{Traces: &operatorv1alpha1.OTLPEndpoints{
				GRPC: "http://traces.telemetry-system:4317",
				HTTP: "http://traces.telemetry-system:4318",
			}},
		},
		{
			name:                 "logs component check error",
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerError:     fmt.Errorf("logs check error"),
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			expectError:          true,
		},
		{
			name: "metrics component check error",
			config: &Config{
				Metrics: MetricsConfig{Enabled: true},
			},
			telemetry:           &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:   &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
			metricsCheckerError: fmt.Errorf("metrics check error"),
			tracesCheckerReturn: &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonTraceGatewayDeploymentReady},
			expectedState:       operatorv1alpha1.StateReady,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
			},
			expectError: true,
		},
		{
			name: "traces component check error",
			config: &Config{
				Metrics: MetricsConfig{Enabled: true},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			tracesCheckerError:   fmt.Errorf("traces check error"),
			expectedState:        operatorv1alpha1.StateReady,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: reconciler.ReasonMetricGatewayDeploymentReady},
			},
			expectError: true,
		},
		{
			name: "deleting with no dependent resources",
			telemetry: &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "default",
					DeletionTimestamp: pointerFrom(metav1.Now()),
				},
			},
			expectedState: operatorv1alpha1.StateDeleting,
		},
		{
			name: "deleting with dependent resources",
			telemetry: &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "default",
					DeletionTimestamp: pointerFrom(metav1.Now()),
				},
			},
			resources: []client.Object{
				pointerFrom(testutils.NewTracePipelineBuilder().Build()),
			},
			expectedState: operatorv1alpha1.StateError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			_ = operatorv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			require.NoError(t, fakeClient.Create(context.Background(), tt.telemetry))
			for _, res := range tt.resources {
				require.NoError(t, fakeClient.Create(context.Background(), res))
			}

			mockLogsChecker := &mocks.ComponentHealthChecker{}
			mockMetricsChecker := &mocks.ComponentHealthChecker{}
			mockTracesChecker := &mocks.ComponentHealthChecker{}
			mockLogsChecker.On("Check", mock.Anything, mock.Anything).Return(tt.logsCheckerReturn, tt.logsCheckerError)
			mockMetricsChecker.On("Check", mock.Anything, mock.Anything).Return(tt.metricsCheckerReturn, tt.metricsCheckerError)
			mockTracesChecker.On("Check", mock.Anything, mock.Anything).Return(tt.tracesCheckerReturn, tt.tracesCheckerError)

			r := &Reconciler{
				Client: fakeClient,
				Scheme: scheme,
				healthCheckers: healthCheckers{
					logs:    mockLogsChecker,
					metrics: mockMetricsChecker,
					traces:  mockTracesChecker,
				},
			}
			if tt.config != nil {
				r.config = *tt.config
			}

			// Act
			err := r.updateStatus(context.Background(), tt.telemetry)

			// Assert
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedState, tt.telemetry.Status.State)
			require.Len(t, tt.telemetry.Status.Conditions, len(tt.expectedConditions))
			for i, expectedCond := range tt.expectedConditions {
				actualCond := tt.telemetry.Status.Conditions[i]
				require.Equal(t, expectedCond.Type, actualCond.Type)
				require.Equal(t, expectedCond.Status, actualCond.Status)
				require.Equal(t, expectedCond.Reason, actualCond.Reason)
				require.Equal(t, expectedCond.Message, actualCond.Message)
				require.NotZero(t, actualCond.LastTransitionTime)
			}
			require.Equal(t, tt.expectedEndpoints, tt.telemetry.Status.GatewayEndpoints)
		})
	}
}

func pointerFrom[T any](value T) *T {
	return &value
}
