package metrics

import (
	"testing"

	. "github.com/onsi/gomega"
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
	suite.RegisterTestCase(t, suite.LabelMetricsSetA)

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
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.MetricPipelineHealthy(t, pipelineName)

	assert.DeploymentReady(t, types.NamespacedName{Name: deploymentName, Namespace: mockNs})

	agentMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricAgentMetricsService.Namespace, kitkyma.MetricAgentMetricsService.Name, "metrics", ports.Metrics)
	assert.EmitsOTelCollectorMetrics(t, agentMetricsURL)
	assert.BackendDataEventuallyMatches(t, backend,
		HaveFlatMetrics(
			ContainElement(HaveResourceAttributes(SatisfyAll(
				HaveKey("cloud.region"),
				HaveKey("cloud.availability_zone"),
				HaveKey("host.type"),
				HaveKey("host.arch"),
				HaveKey("k8s.cluster.name"),
				HaveKey("cloud.provider"),
			))),
		), "Could not find metrics matching resource attributes",
	)
}
