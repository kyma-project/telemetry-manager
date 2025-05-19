package shared

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
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

const maxNumberOfLogPipelines = 5

func TestMultiPipelineMaxPipeline_OTel(t *testing.T) {
	tests := []struct {
		label               string
		input               telemetryv1alpha1.LogPipelineInput
		logGeneratorBuilder func(namespace string) client.Object
		expectAgent         bool
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineApplicationInput(),
			logGeneratorBuilder: func(namespace string) client.Object {
				return loggen.New(namespace).K8sObject()
			},
			expectAgent: true,
		},
		{
			label: suite.LabelLogGateway,
			input: testutils.BuildLogPipelineOTLPInput(),
			logGeneratorBuilder: func(namespace string) client.Object {
				return telemetrygen.NewDeployment(namespace, telemetrygen.SignalTypeLogs).K8sObject()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label, suite.LabelSkip) // skipping for now, as we need to execute this test in isolation

			var (
				uniquePrefix = unique.Prefix(tc.label)
				backendNs    = uniquePrefix("backend")
				generatorNs  = uniquePrefix("gen")
				pipelineBase = uniquePrefix("pipeline")
				pipelines    []client.Object
			)

			backend := backend.New(backendNs, backend.SignalTypeLogsOTel)
			backendExportURL := backend.ExportURL(suite.ProxyClient)

			for i := range maxNumberOfLogPipelines {
				pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
				pipeline := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithInput(tc.input).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				pipelines = append(pipelines, &pipeline)
			}

			invalidPipelineName := fmt.Sprintf("%s-invalid", pipelineBase)
			invalidPipeline := testutils.NewLogPipelineBuilder().
				WithName(invalidPipelineName).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(generatorNs).K8sObject(),
				tc.logGeneratorBuilder(generatorNs),
			}
			resources = append(resources, backend.K8sObjects()...)
			resources = append(resources, pipelines...)

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

			assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
			assert.DeploymentReady(t.Context(), suite.K8sClient, kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.LogAgentName)
			}

			t.Log("Asserting 5 pipelines are healthy")

			for _, pipeline := range pipelines {
				assert.OTelLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline.GetName())
			}

			t.Log("Attempting to create the 6th pipeline and expecting failure")

			err := kitk8s.CreateObjects(t.Context(), suite.K8sClient, &invalidPipeline)
			require.Error(t, err, "Expected invalid pipeline creation to fail")

			t.Log("Verifying logs are delivered for valid pipelines")
			assert.OTelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
		})
	}
}

func TestMultiPipelineMaxPipeline_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit, suite.LabelSkip) // skipping for now, as we need to execute this test in isolation

	var (
		uniquePrefix = unique.Prefix()
		backendNs    = uniquePrefix("backend")
		generatorNs  = uniquePrefix("gen")
		pipelineBase = uniquePrefix("pipeline")
		pipelines    []client.Object
	)

	backend := backend.New(backendNs, backend.SignalTypeLogsFluentBit)
	backendExportURL := backend.ExportURL(suite.ProxyClient)

	for i := range maxNumberOfLogPipelines {
		pipelineName := fmt.Sprintf("%s-%d", pipelineBase, i)
		pipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInput(true).
			WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
			Build()
		pipelines = append(pipelines, &pipeline)
	}

	invalidPipelineName := fmt.Sprintf("%s-invalid", pipelineBase)
	invalidPipeline := testutils.NewLogPipelineBuilder().
		WithName(invalidPipelineName).
		WithApplicationInput(true).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(generatorNs).K8sObject(),
		loggen.New(generatorNs).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, pipelines...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), suite.K8sClient, resources...))

	assert.DeploymentReady(t.Context(), suite.K8sClient, backend.NamespacedName())
	assert.DaemonSetReady(t.Context(), suite.K8sClient, kitkyma.FluentBitDaemonSetName)

	t.Log("Asserting 5 pipelines are healthy")

	for _, pipeline := range pipelines {
		assert.FluentBitLogPipelineHealthy(t.Context(), suite.K8sClient, pipeline.GetName())
	}

	t.Log("Attempting to create the 6th pipeline and expecting failure")

	err := kitk8s.CreateObjects(t.Context(), suite.K8sClient, &invalidPipeline)
	require.Error(t, err, "Expected exceeding pipeline creation to fail")

	t.Log("Verifying logs are delivered for valid pipelines")
	assert.FluentBitLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, generatorNs)
}
