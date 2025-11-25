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

func TestLogComponentsCheck(t *testing.T) {
	healthyAgentCond := metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonAgentReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonAgentConfigured}

	tests := []struct {
		name                     string
		pipelines                []telemetryv1alpha1.LogPipeline
		tracePipelines           []telemetryv1alpha1.TracePipeline
		metricPipelines          []telemetryv1alpha1.MetricPipeline
		telemetryInDeletion      bool
		flowHealthProbingEnabled bool
		expectedCondition        *metav1.Condition
	}{
		{
			name:                "should be healthy if no pipelines deployed",
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should be healthy if all pipelines running",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "True",
				Reason:  conditions.ReasonComponentsRunning,
				Message: "All log components are running",
			},
		},
		{
			name: "should not be healthy if one pipeline refs missing secret",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
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
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  conditions.ReasonReferencedSecretMissing,
				Message: "One or more referenced Secrets are missing",
			},
		},
		{
			name: "should not be healthy if one pipeline waiting for fluent bit",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeAgentHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonAgentNotReady,
						Message: conditions.MessageForFluentBitLogPipeline(conditions.ReasonAgentNotReady),
					}).
					WithStatusCondition(configGeneratedCond).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "AgentNotReady",
				Message: "Log agent DaemonSet is not ready",
			},
		},
		{
			name: "should prioritize unready ConfigGenerated reason over AgentHealthy reason",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeAgentHealthy,
						Status: metav1.ConditionFalse,
						Reason: conditions.ReasonAgentNotReady,
					}).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
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
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  conditions.ReasonReferencedSecretMissing,
				Message: "One or more referenced Secrets are missing",
			},
		},
		{
			name: "should block deletion if there are existing log pipelines",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("foo").Build(),
				testutils.NewLogPipelineBuilder().WithName("bar").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (bar,foo)",
			},
		},
		{
			name: "should not block deletion if there are existing trace pipelines",
			tracePipelines: []telemetryv1alpha1.TracePipeline{
				testutils.NewTracePipelineBuilder().WithName("foo").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should not block deletion if there are existing metric pipelines",
			metricPipelines: []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("foo").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "True",
				Reason:  "NoPipelineDeployed",
				Message: "No pipelines have been deployed",
			},
		},
		{
			name: "should block deletion if there are existing pipelines",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("foo").Build(),
				testutils.NewLogPipelineBuilder().WithName("baz").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (baz,foo)",
			},
		},
		{
			name: "should be healthy if telemetry flow probing enabled and healthy",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeFlowHealthy,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonSelfMonFlowHealthy,
					}).
					Build(),
			},
			flowHealthProbingEnabled: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "True",
				Reason:  conditions.ReasonComponentsRunning,
				Message: "All log components are running",
			},
		},
		{
			name: "should not be healthy if telemetry flow probing enabled and not healthy",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeFlowHealthy,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonSelfMonAgentNoLogsDelivered,
						Message: conditions.MessageForFluentBitLogPipeline(conditions.ReasonSelfMonAgentNoLogsDelivered),
					}).
					Build(),
			},
			flowHealthProbingEnabled: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "AgentNoLogsDelivered",
				Message: "Backend is not reachable or rejecting logs. Logs are buffered and not yet dropped. See troubleshooting: " + conditions.LinkFluentBitNoLogsArriveAtBackend,
			},
		},
		{
			name: "should show tlsCertExpert if one pipeline has invalid tls cert and the other pipeline has an about to expire cert",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  conditions.ReasonTLSCertificateExpired,
						Message: "TLS certificate is expired",
					}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:   conditions.TypeConfigurationGenerated,
						Status: metav1.ConditionTrue,
						Reason: conditions.ReasonTLSCertificateAboutToExpire,
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "TLSCertificateExpired",
				Message: "TLS certificate is expired",
			},
		},
		{
			name: "should show tlsCert is about to expire if one of the pipelines has tls cert which is about to expire",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
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
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "True",
				Reason:  "TLSCertificateAboutToExpire",
				Message: "TLS certificate is about to expire",
			},
		},
		{
			name: "should not be healthy of one pipeline has a failed request to the Kubernetes API server during validation",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{
						Type:    conditions.TypeConfigurationGenerated,
						Status:  metav1.ConditionFalse,
						Reason:  "ValidationFailed",
						Message: "Pipeline validation failed due to an error from the Kubernetes API server",
					}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
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

			m := &logComponentsChecker{
				client: fakeClient,
			}

			condition, err := m.Check(t.Context(), test.telemetryInDeletion)
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
