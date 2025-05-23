package shared

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken_OTel(t *testing.T) {
	tests := []struct {
		label               string
		inputBuilder        func(includeNs string) telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(ns string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return loggen.New(ns).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			inputBuilder: func(includeNs string) telemetryv1alpha1.LogPipelineInput {
				return testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))
			},
			logGeneratorBuilder: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label, suite.LabelSkip) // FIXME: Currently failing (not implemented)

			var (
				uniquePrefix   = unique.Prefix(tc.label)
				backendNs      = uniquePrefix("backend")
				genNs          = uniquePrefix("gen")
				goodPipeline   = uniquePrefix("good")
				brokenPipeline = uniquePrefix("broken")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipelineGood := testutils.NewLogPipelineBuilder().
				WithName(goodPipeline).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			pipelineBroken := testutils.NewLogPipelineBuilder().
				WithName(brokenPipeline).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("dummy", "dummy", "user", "password")). // broken pipeline references a secret that does not exist
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipelineGood,
				&pipelineBroken,
				tc.logGeneratorBuilder(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

			assert.DeploymentReady(t.Context(), backend.NamespacedName())
			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t.Context(), pipelineGood.Name)
			assert.LogPipelineHasCondition(t.Context(), pipelineBroken.Name, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})

			assert.OTelLogsFromNamespaceDelivered(t.Context(), backend, genNs)
		})
	}
}

func TestMultiPipelineBroken_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix   = unique.Prefix()
		backendNs      = uniquePrefix("backend")
		genNs          = uniquePrefix("gen")
		goodPipeline   = uniquePrefix("good")
		brokenPipeline = uniquePrefix("broken")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	pipelineGood := testutils.NewLogPipelineBuilder().
		WithName(goodPipeline).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	pipelineBroken := testutils.NewLogPipelineBuilder().
		WithName(brokenPipeline).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHostFromSecret("dummy", "dummy", "dummy")). // broken pipeline ref
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipelineGood,
		&pipelineBroken,
		loggen.New(genNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), pipelineGood.Name)
	assert.LogPipelineHasCondition(t.Context(), pipelineBroken.Name, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, genNs)
}
