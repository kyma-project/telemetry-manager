package shared

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	fbports "github.com/kyma-project/telemetry-manager/internal/fluentbit/ports"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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

func TestMetricsEndpoint_OTel(t *testing.T) {
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
				return stdloggen.NewDeployment(namespace).K8sObject()
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
			suite.RegisterTestCase(t, tc.label)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix("pipeline")
				generatorNs  = uniquePrefix("gen")
				backendNs    = uniquePrefix("backend")
			)

			backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsOTel)

			pipeline := testutils.NewLogPipelineBuilder().
				WithName(pipelineName).
				WithInput(tc.input).
				WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
				Build()

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(generatorNs).K8sObject(),
				&pipeline,
				tc.logGeneratorBuilder(generatorNs),
			}

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

			assert.DeploymentReady(t.Context(), kitkyma.LogGatewayName)

			if tc.expectAgent {
				assert.DaemonSetReady(t.Context(), kitkyma.LogAgentName)
				agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.LogAgentMetricsService.Namespace, kitkyma.LogAgentMetricsService.Name, "metrics", ports.Metrics)
				assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)
			}

			gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.LogGatewayMetricsService.Namespace, kitkyma.LogGatewayMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(t, gatewayMetricsURL)
		})
	}
}

func TestMetricsEndpoint_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeLogsFluentBit)

	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&pipeline,
	}

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	require.NoError(t, kitk8s.CreateObjects(t.Context(), resources...))

	assert.DaemonSetReady(t.Context(), kitkyma.FluentBitDaemonSetName)

	assert.FluentBitLogPipelineHealthy(t, pipelineName)
	assert.LogPipelineUnsupportedMode(t, pipelineName, false)

	fluentBitMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.FluentBitMetricsService.Namespace, kitkyma.FluentBitMetricsService.Name, "api/v1/metrics/prometheus", fbports.HTTP)
	assert.EmitsFluentBitMetrics(t, fluentBitMetricsURL)
}
