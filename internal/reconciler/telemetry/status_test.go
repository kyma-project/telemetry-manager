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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
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
				Metrics: MetricsConfig{OTLPServiceName: "metrics", Namespace: "telemetry-system"},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateReady,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{
				Traces: &operatorv1alpha1.OTLPEndpoints{
					GRPC: "http://traces.telemetry-system:4317",
					HTTP: "http://traces.telemetry-system:4318",
				},
				Metrics: &operatorv1alpha1.OTLPEndpoints{
					GRPC: "http://metrics.telemetry-system:4317",
					HTTP: "http://metrics.telemetry-system:4318",
				}},
		},
		{
			name: "non trace components are unhealthy",
			config: &Config{
				Traces:  TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
				Metrics: MetricsConfig{OTLPServiceName: "metrics", Namespace: "telemetry-system"},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{
				Traces: &operatorv1alpha1.OTLPEndpoints{
					GRPC: "http://traces.telemetry-system:4317",
					HTTP: "http://traces.telemetry-system:4318",
				},
				Metrics: &operatorv1alpha1.OTLPEndpoints{
					GRPC: "http://metrics.telemetry-system:4317",
					HTTP: "http://metrics.telemetry-system:4318",
				}},
		},
		{
			name: "trace components are unhealthy",
			config: &Config{
				Metrics: MetricsConfig{OTLPServiceName: "metrics", Namespace: "telemetry-system"},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonTraceGatewayDeploymentNotReady},
			expectedState:        operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonTraceGatewayDeploymentNotReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{Metrics: &operatorv1alpha1.OTLPEndpoints{
				GRPC: "http://metrics.telemetry-system:4317",
				HTTP: "http://metrics.telemetry-system:4318",
			}},
		},
		{
			name: "metrics are unhealthy",
			config: &Config{
				Traces: TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonMetricGatewayDeploymentNotReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonMetricGatewayDeploymentNotReady},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
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
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricGatewayDeploymentReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			expectError:          true,
		},
		{
			name:                "metrics component check error",
			telemetry:           &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:   &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerError: fmt.Errorf("metrics check error"),
			tracesCheckerReturn: &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			},
			expectError: true,
		},
		{
			name:                 "traces component check error",
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricGatewayDeploymentReady},
			tracesCheckerError:   fmt.Errorf("traces check error"),
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricGatewayDeploymentReady},
			},
			expectError: true,
		},
		{
			name: "deleting with no dependent resources",
			config: &Config{
				Traces:  TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
				Metrics: MetricsConfig{OTLPServiceName: "metrics", Namespace: "telemetry-system"},
			},
			telemetry: &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "default",
					DeletionTimestamp: pointerFrom(metav1.Now()),
					Finalizers:        []string{"telemetry.kyma-project.io/finalizer"},
				},
			},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			expectedState:        operatorv1alpha1.StateDeleting,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{
				Traces: &operatorv1alpha1.OTLPEndpoints{
					GRPC: "http://traces.telemetry-system:4317",
					HTTP: "http://traces.telemetry-system:4318",
				}, Metrics: &operatorv1alpha1.OTLPEndpoints{
					GRPC: "http://metrics.telemetry-system:4317",
					HTTP: "http://metrics.telemetry-system:4318",
				}},
		},
		{
			name: "deleting with dependent resources",
			config: &Config{
				Traces:  TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
				Metrics: MetricsConfig{OTLPServiceName: "metrics", Namespace: "telemetry-system"},
			},
			telemetry: &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "default",
					DeletionTimestamp: pointerFrom(metav1.Now()),
					Finalizers:        []string{"telemetry.kyma-project.io/finalizer"},
				},
			},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonResourceBlocksDeletion},
			resources: []client.Object{
				pointerFrom(testutils.NewTracePipelineBuilder().Build()),
			},
			expectedState: operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonMetricComponentsRunning},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonResourceBlocksDeletion},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{Metrics: &operatorv1alpha1.OTLPEndpoints{
				GRPC: "http://metrics.telemetry-system:4317",
				HTTP: "http://metrics.telemetry-system:4318",
			}},
		},
		{
			name: "metric agent is unhealthy",
			config: &Config{
				Traces: TracesConfig{OTLPServiceName: "traces", Namespace: "telemetry-system"},
			},
			telemetry:            &operatorv1alpha1.Telemetry{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			logsCheckerReturn:    &metav1.Condition{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
			metricsCheckerReturn: &metav1.Condition{Type: "MetricComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonMetricAgentDaemonSetNotReady},
			tracesCheckerReturn:  &metav1.Condition{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			resources: []client.Object{
				pointerFrom(testutils.NewTracePipelineBuilder().Build()),
			},
			expectedState: operatorv1alpha1.StateWarning,
			expectedConditions: []metav1.Condition{
				{Type: "LogComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady},
				{Type: "MetricComponentsHealthy", Status: metav1.ConditionFalse, Reason: conditions.ReasonMetricAgentDaemonSetNotReady},
				{Type: "TraceComponentsHealthy", Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentReady},
			},
			expectedEndpoints: operatorv1alpha1.GatewayEndpoints{Traces: &operatorv1alpha1.OTLPEndpoints{
				GRPC: "http://traces.telemetry-system:4317",
				HTTP: "http://traces.telemetry-system:4318",
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)
			_ = operatorv1alpha1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.telemetry).WithStatusSubresource(tt.telemetry).Build()
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
