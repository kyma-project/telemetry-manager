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
	healthyAgentCond := metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionTrue, Reason: conditions.ReasonDaemonSetReady}
	configGeneratedCond := metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonConfigurationGenerated}
	runningCondition := metav1.Condition{Type: conditions.TypeRunning, Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSReady}

	tests := []struct {
		name                string
		pipelines           []telemetryv1alpha1.LogPipeline
		parsers             []telemetryv1alpha1.LogParser
		telemetryInDeletion bool
		expectedCondition   *metav1.Condition
	}{
		{
			name:                "should be healthy if no pipelines deployed",
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "LogComponentsHealthy",
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
				Type:    "LogComponentsHealthy",
				Status:  "True",
				Reason:  "LogComponentsRunning",
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
				Type:    "LogComponentsHealthy",
				Status:  "False",
				Reason:  "LogPipelineReferencedSecretMissing",
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
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDaemonSetNotReady}).
					WithStatusCondition(configGeneratedCond).
					WithStatusCondition(metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonFluentBitDSNotReady}).
					Build(),
			},
			telemetryInDeletion: false,
			expectedCondition: &metav1.Condition{
				Type:    "LogComponentsHealthy",
				Status:  "False",
				Reason:  "FluentBitDaemonSetNotReady",
				Message: "Fluent Bit DaemonSet is not ready",
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
				Type:    "LogComponentsHealthy",
				Status:  "False",
				Reason:  "UnsupportedLokiOutput",
				Message: "grafana-loki output is not supported anymore. For integration with a custom Loki installation, use the `custom` output and follow https://kyma-project.io/#/telemetry-manager/user/integration/loki/README",
			},
		},
		{
			name: "should prioritize unready fluent bit reason over missing secret",
			pipelines: []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().
					WithStatusCondition(metav1.Condition{Type: conditions.TypeAgentHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDaemonSetNotReady}).
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
				Type:    "LogComponentsHealthy",
				Status:  "False",
				Reason:  "FluentBitDaemonSetNotReady",
				Message: "Fluent Bit DaemonSet is not ready",
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
				Type:    "LogComponentsHealthy",
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
				Type:    "LogComponentsHealthy",
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
				Type:    "LogComponentsHealthy",
				Status:  "False",
				Reason:  "ResourceBlocksDeletion",
				Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (baz,foo), LogParsers (bar)",
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
				client: fakeClient,
			}

			condition, err := m.Check(context.Background(), test.telemetryInDeletion)
			require.NoError(t, err)
			require.Equal(t, test.expectedCondition, condition)
		})
	}
}
