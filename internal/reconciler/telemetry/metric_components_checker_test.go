package telemetry

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMetricComponentsCheck(t *testing.T) {
	healthyGatewayCond := metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonGatewayReady}
	healthyAgentCond := metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonAgentReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonGatewayConfigured}

	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.MetricPipeline
		tracePipelines      []telemetryv1alpha1.TracePipeline
		logPipelines        []telemetryv1alpha1.LogPipeline
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
						Message: "Maximum pipeline count limit exceeded",
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
			name: "should block deletion if there are existing metric pipelines",
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
			name: "should not block deletion if there are existing trace pipelines",
			tracePipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("foo").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should not block deletion if there are existing log pipelines",
			logPipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("foo").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
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
				Message: "Metric gateway is unable to receive metrics at current rate. See troubleshooting: " + conditions.LinkGatewayThrottling,
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
				Message: "Metric gateway is unable to receive metrics at current rate. See troubleshooting: " + conditions.LinkGatewayThrottling,
			},
		},
		{
			name: "should not be healthy if telemetry flow probing enabled and metric agent flow is not healthy",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeFlowHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonSelfMonAgentAllDataDropped,
						Message: conditions.MessageForMetricPipeline(conditions.ReasonSelfMonAgentAllDataDropped),
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "AgentAllTelemetryDataDropped",
				Message: "Backend is not reachable or rejecting metrics. All metrics are dropped. See troubleshooting: " + conditions.LinkNoDataArriveAtBackend,
			},
		},
		{
			name: "should show tlsCertExpert if one pipeline has invalid tls cert and the other pipeline has an about to expire cert",
			pipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyGatewayCond).
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeConfigurationGenerated,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonTLSCertificateAboutToExpire,
					}).
					Build(),
				testutils.NewMetricPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonTLSCertificateExpired,
						Message: "TLS certificate is expired",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "TLSCertificateExpired",
				Message: "TLS certificate is expired",
			},
		},
		{
			name: "should show tlsCert is about to expire if one of the pipelines has tls cert which is about to expire",
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
						Status:  metav1.ConditionTrue,
						Reason:  conditions.ReasonTLSCertificateAboutToExpire,
						Message: "TLS certificate is about to expire",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "True",
				Reason:  "TLSCertificateAboutToExpire",
				Message: "TLS certificate is about to expire",
			},
		},
		{
			name: "should not be healthy of one pipeline has a failed request to the Kubernetes API server during validation",
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
						Status:  "False",
						Reason:  "ValidationFailed",
						Message: "Pipeline validation failed due to an error from the Kubernetes API server",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeMetricComponentsHealthy,
				Status:  "False",
				Reason:  "ValidationFailed",
				Message: "Pipeline validation failed due to an error from the Kubernetes API server",
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

			condition, err := m.Check(t.Context(), test.telemetryInDeletion)
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
