package metrics

import (
	"context"
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestCloudProviderAttributes(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

	var (
		uniquePrefix   = unique.Prefix()
		pipelineName   = uniquePrefix("resource-metrics")
		backendName    = uniquePrefix("resource-metrics")
		deploymentName = uniquePrefix("deployment")
		genNs          = uniquePrefix("gen")
		mockNs         = uniquePrefix("mock")
	)

	backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(backendName))
	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithRuntimeInput(true).
		WithRuntimeInputContainerMetrics(true).
		WithRuntimeInputPodMetrics(true).
		WithRuntimeInputNodeMetrics(true).
		WithRuntimeInputVolumeMetrics(true).
		WithRuntimeInputDeploymentMetrics(false).
		WithRuntimeInputStatefulSetMetrics(false).
		WithRuntimeInputDaemonSetMetrics(false).
		WithRuntimeInputJobMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()
	metricProducer := prommetricgen.New(genNs)

	podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics)

	deployment := kitk8s.NewDeployment(deploymentName, mockNs).WithPodSpec(podSpec).WithLabel("name", deploymentName).K8sObject()

	resources := []client.Object{
		kitk8s.NewNamespace(mockNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&pipeline,
		metricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		metricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		deployment,
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.MetricPipelineHealthy(t.Context(), pipelineName)
	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DeploymentReady(t.Context(), backend.NamespacedName())
	assert.DeploymentReady(t.Context(), types.NamespacedName{Name: deploymentName, Namespace: mockNs})
	agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
	assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)
	backendContainsDesiredCloudResourceAttributes(t, backend, "cloud.region")
	backendContainsDesiredCloudResourceAttributes(t, backend, "cloud.availability_zone")
	backendContainsDesiredCloudResourceAttributes(t, backend, "host.type")
	backendContainsDesiredCloudResourceAttributes(t, backend, "host.arch")
	backendContainsDesiredCloudResourceAttributes(t, backend, "k8s.cluster.name")
	backendContainsDesiredCloudResourceAttributes(t, backend, "cloud.provider")
}

func backendContainsDesiredCloudResourceAttributes(t *testing.T, backend *kitbackend.Backend, attribute string) {
	t.Helper()

	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			ContainElement(SatisfyAll(
				HaveResourceAttributes(HaveKey(attribute)),
			)),
		), fmt.Sprintf("could not find metrics matching resource attribute %s", attribute))
}
