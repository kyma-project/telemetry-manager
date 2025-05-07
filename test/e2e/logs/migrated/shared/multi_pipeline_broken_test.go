package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMultiPipelineBroken_OTel(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name                string
		input               telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(namespace string) client.Object
		expectAgent         bool
	}{
		{
			name:  "agent",
			input: testutils.BuildLogPipelineApplicationInput(),
			logGeneratorBuilder: func(namespace string) client.Object {
				return loggen.New(namespace).K8sObject()
			},
			expectAgent: true,
		},
		{
			name:  "gateway",
			input: testutils.BuildLogPipelineOTLPInput(),
			logGeneratorBuilder: func(namespace string) client.Object {
				return telemetrygen.NewDeployment(namespace, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				uniquePrefix   = unique.Prefix(tc.name)
				backendNs      = uniquePrefix("backend")
				generatorNs    = uniquePrefix("gen")
				goodPipeline   = uniquePrefix("good")
				brokenPipeline = uniquePrefix("broken")
			)

			backend := backend.New(backendNs, backend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)

			logPipelineGood := testutils.NewLogPipelineBuilder().
				WithName(goodPipeline).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			logPipelineBroken := testutils.NewLogPipelineBuilder().
				WithName(brokenPipeline).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPBasicAuthFromSecret("dummy", "dummy", "user", "password")). // broken pipeline references a secret that does not exist
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(generatorNs).K8sObject(),
				&logPipelineGood,
				&logPipelineBroken,
				tc.logGeneratorBuilder(generatorNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

			assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
			if tc.expectAgent {
				assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, logPipelineGood.Name)
			// TODO(skhalash): Uncomment when validation is implemented
			// assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, logPipelineBroken.Name, metav1.Condition{
			// 	Type:   conditions.TypeConfigurationGenerated,
			// 	Status: metav1.ConditionFalse,
			// 	Reason: conditions.ReasonReferencedSecretMissing,
			// })

			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
		})
	}
}

func TestMultiPipelineBroken_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	var (
		uniquePrefix   = unique.Prefix()
		backendNs      = uniquePrefix("backend")
		generatorNs    = uniquePrefix("gen")
		goodPipeline   = uniquePrefix("good")
		brokenPipeline = uniquePrefix("broken")
	)

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	logPipelineGood := testutils.NewLogPipelineBuilder().
		WithName(goodPipeline).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	logPipelineBroken := testutils.NewLogPipelineBuilder().
		WithName(brokenPipeline).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHostFromSecret("dummy", "dummy", "dummy")). // broken pipeline ref
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		&logPipelineGood,
		&logPipelineBroken,
		loggen.New(generatorNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(suite.Ctx, suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, logPipelineGood.Name)
	assert.LogPipelineHasCondition(t.Context(), suite.K8sClient, logPipelineBroken.Name, metav1.Condition{
		Type:   conditions.TypeConfigurationGenerated,
		Status: metav1.ConditionFalse,
		Reason: conditions.ReasonReferencedSecretMissing,
	})

	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
}
