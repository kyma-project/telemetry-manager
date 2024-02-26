package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMetricComponentsCheck(t *testing.T) {
	healthyGatewayCond := metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonDeploymentReady}
	healthyAgentCond := metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonDaemonSetReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonConfigurationGenerated}

	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.MetricPipeline
		telemetryInDeletion bool
		expectedCondition   *metav1.Condition
	}{
		{
			name:                "should be healthy if no pipelines deployed",
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should be healthy if all pipelines running",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "True",
				Reason:  "MetricComponentsRunning",
				Message: "All metric components are running",
			},
		},
		{
			name: "should not be healthy if one pipeline refs missing secret",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonReferencedSecretMissing}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MetricPipelineReferencedSecretMissing",
				Message: "One or more referenced Secrets are missing",
			},
		},
		{
			name: "should not be healthy if one pipeline waiting for gateway",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDeploymentNotReady}).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MetricGatewayDeploymentNotReady",
				Message: "Metric gateway Deployment is not ready",
			},
		},
		{
			name: "should not be healthy if one pipeline waiting for agent",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDaemonSetNotReady}).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MetricAgentDaemonSetNotReady",
				Message: "Metric agent DaemonSet is not ready",
			},
		},
		{
			name: "should not be healthy if max pipelines exceeded",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonMaxPipelinesExceeded}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MaxPipelinesExceeded",
				Message: "Maximum pipeline count limit exceeded",
			},
		},
		{
			name: "should prioritize unhealthy gateway reason over unhealthy agent",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDeploymentNotReady}).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDaemonSetNotReady}).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDeploymentNotReady}).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDaemonSetNotReady}).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "MetricGatewayDeploymentNotReady",
				Message: "Metric gateway Deployment is not ready",
			},
		},
		{
			name: "should block deletion if there are existing pipelines",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("foo").Build(),
				testutils.NewMetricPipelineBuilder().WithName("bar").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    "MetricComponentsHealthy",
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: MetricPipelines (bar,foo)",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			_ = clientgoscheme.AddToScheme(scheme)
			_ = telemetryv1alpha1.AddToScheme(scheme)

			b := fake.NewClientBuilder().WithScheme(scheme)
			for i := range test.pipelines {
				b.WithObjects(&test.pipelines[i])
			}
			fakeClient := b.Build()

			m := &metricComponentsChecker{
				client: fakeClient,
			}

			condition, err := m.Check(context.Background(), test.telemetryInDeletion)
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
