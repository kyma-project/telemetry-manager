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
	healthyGatewayCond := metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonGatewayReady}
	healthyAgentCond := metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonAgentReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonGatewayConfigured}

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
				Type:    conditions.TypeMetricComponentsHealthy,
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
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "True",
				Reason:  conditions.ReasonComponentsRunning,
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
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonReferencedSecretMissing,
						Message: "One or more referenced Secrets are missing",
					}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  conditions.ReasonReferencedSecretMissing,
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
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeGatewayHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonGatewayNotReady,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonGatewayNotReady),
					}).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "GatewayNotReady",
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
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeAgentHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonAgentNotReady,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonAgentNotReady),
					}).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "AgentNotReady",
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
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonMaxPipelinesExceeded,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonMaxPipelinesExceeded),
					}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "MaxPipelinesExceeded",
				Message: "Maximum pipeline count limit exceeded",
			},
		},
		{
			name: "should prioritize unhealthy gateway reason over unhealthy agent",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeGatewayHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonGatewayNotReady,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonGatewayNotReady),
					}).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeAgentHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonAgentNotReady,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonAgentNotReady),
					}).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeGatewayHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonGatewayNotReady,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonGatewayNotReady),
					}).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeAgentHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonAgentNotReady,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonAgentNotReady),
					}).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "GatewayNotReady",
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
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: MetricPipelines (bar,foo)",
			},
		},
		{
			name: "should be healthy if telemetry flow probing enabled and not healthy",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeFlowHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonSelfMonGatewayThrottling,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonSelfMonGatewayThrottling),
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "GatewayThrottling",
				Message: "Metric gateway is unable to receive metrics at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=gateway-throttling",
			},
		},
		{
			name: "should not be healthy if telemetry flow probing enabled and healthy",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeFlowHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonSelfMonGatewayThrottling,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonSelfMonGatewayThrottling),
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "GatewayThrottling",
				Message: "Metric gateway is unable to receive metrics at current rate. See troubleshooting: https://kyma-project.io/#/telemetry-manager/user/04-metrics?id=gateway-throttling",
			},
		},
		{
			name: "should return show tlsCertInvalid if one of the pipelines has invalid tls cert",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonTLSConfigurationInvalid,
						Message: "TLS configuration invalid: unable to decode pem blocks",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "TLSConfigurationInvalid",
				Message: "TLS configuration invalid: unable to decode pem blocks",
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
