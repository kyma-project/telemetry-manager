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

func TestLogComponentsCheck(t *testing.T) {
	healthyAgentCond := metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonAgentReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonAgentConfigured}
	runningCondition := metav1.Condition{Type: conditions.TypeRunning, Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady}

	tests := []struct {
		name                     string
		pipelines                []telemetryv1alpha1.LogPipeline
		parsers                  []telemetryv1alpha1.LogParser
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
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady}).
					WithStatusCondition(runningCondition).
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
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonReferencedSecretMissing}).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonReferencedSecretMissing}).
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
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonAgentNotReady}).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSNotReady}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "AgentNotReady",
				Message: "Fluent Bit agent DaemonSet is not ready",
			},
		},
		{
			name: "should not be healthy if one pipeline has Loki output defined",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonUnsupportedLokiOutput}).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonUnsupportedLokiOutput}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "UnsupportedLokiOutput",
				Message: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
			},
		},
		{
			name: "should prioritize unready ConfigGenerated reason over AgentHealthy reason",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonAgentNotReady}).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSNotReady}).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonReferencedSecretMissing}).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonReferencedSecretMissing}).
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
			name: "should block deletion if there are existing pipelines",
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
			name: "should block deletion if there are existing parsers",
			parsers: []telemetryv1alpha1.LogParser{
				testutils.NewLogParsersBuilder().WithName("foo").Build(),
				testutils.NewLogParsersBuilder().WithName("bar").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogParsers (bar,foo)",
			},
		},
		{
			name: "should block deletion if there are existing pipelines and parsers",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("foo").Build(),
				testutils.NewLogPipelineBuilder().WithName("baz").Build(),
			},
			parsers: []telemetryv1alpha1.LogParser{
				testutils.NewLogParsersBuilder().WithName("bar").Build(),
			},
			telemetryInDeletion: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (baz,foo), LogParsers (bar)",
			},
		},
		{
			name: "should be healthy if telemetry flow probing enabled and healthy",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeFlowHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonSelfMonFlowHealthy}).
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
					WithStatusCondition(metav1.Condition{Type: conditions.TypeFlowHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonSelfMonNoLogsDelivered}).
					Build(),
			},
			flowHealthProbingEnabled: true,
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "NoLogsDelivered",
				Message: "No logs delivered to backend",
			},
		},
		{
			name: "should return show tlsCertificateExpired if one of the pipelines has expired tls cert",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionFalse, Reason: conditions.ReasonFluentBitDSNotReady}).
					WithStatusCondition(runningCondition).
					Build(),
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(healthyAgentCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionFalse, Reason: conditions.ReasonTLSCertificateExpired, Message: "TLS certificate expired on 20.01.2023"}).
					Build(),
			},
			expectedCondition: &metav1.Condition{
				Type:    conditions.TypeLogComponentsHealthy,
				Status:  "False",
				Reason:  "TLSCertificateExpired",
				Message: "TLS certificate expired on 20.01.2023",
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
			for i := range test.parsers {
				b.WithObjects(&test.parsers[i])
			}
			fakeClient := b.Build()

			m := &logComponentsChecker{
				client:                   fakeClient,
				flowHealthProbingEnabled: test.flowHealthProbingEnabled,
			}

			condition, err := m.Check(context.Background(), test.telemetryInDeletion)
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
