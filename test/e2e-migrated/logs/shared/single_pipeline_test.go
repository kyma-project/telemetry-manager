package shared

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestSinglePipeline_OTel(t *testing.T) {
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
				return stdloggen.NewDeployment(ns).K8sObject()
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
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix("pipeline")
				genNs        = uniquePrefix("gen")
				backendNs    = uniquePrefix("backend")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.inputBuilder(genNs)).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				&pipeline,
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
				assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)
			}

			assert.OTelLogPipelineHealthy(t.Context(), pipelineName)
			assert.OTelLogsFromNamespaceDelivered(t.Context(), backend, genNs)
		})
	}
}

func TestSinglePipeline_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		generatorNs  = uniquePrefix("gen")
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		stdloggen.NewDeployment(generatorNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t.Context(), pipelineName)
	assert.LogPipelineUnsupportedMode(t.Context(), pipelineName, false)

	assert.FluentBitLogsFromNamespaceDelivered(t.Context(), backend, generatorNs)
}
