package agent

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestRuntimeNodeNamespace(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricAgentSetB)

	var (
		uniquePrefix        = unique.Prefix()
		includePipelineName = uniquePrefix("include")
		excludePipelineName = uniquePrefix("exclude")
		includeBackendName  = uniquePrefix("be-include")
		excludeBackendName  = uniquePrefix("be-exclude")
		backendNs           = uniquePrefix("backend")
		includeNs           = uniquePrefix("include")
		excludeNs           = uniquePrefix("exclude")
	)

	includeBacked := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(includeBackendName))
	excludeBackend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics, kitbackend.WithName(excludeBackendName))

	includePipeline := testutils.NewMetricPipelineBuilder().
		WithName(includePipelineName).
		WithRuntimeInput(true, testutils.IncludeNamespaces(includeNs)).
		WithRuntimeInputNodeMetrics(true).
		WithRuntimeInputPodMetrics(false).
		WithRuntimeInputContainerMetrics(false).
		WithRuntimeInputVolumeMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(includeBacked.EndpointHTTP())).
		Build()
	excludePipeline := testutils.NewMetricPipelineBuilder().
		WithName(excludePipelineName).
		WithRuntimeInput(true, testutils.ExcludeNamespaces(excludeNs)).
		WithRuntimeInputNodeMetrics(true).
		WithRuntimeInputPodMetrics(false).
		WithRuntimeInputContainerMetrics(false).
		WithRuntimeInputVolumeMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(excludeBackend.EndpointHTTP())).
		Build()

	includeMetricProducer := prommetricgen.New(includeNs)
	excludeMetricProducer := prommetricgen.New(excludeNs)

	resources := []client.Object{
		kitk8sobjects.NewNamespace(includeNs).K8sObject(),
		kitk8sobjects.NewNamespace(excludeNs).K8sObject(),
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		&includePipeline,
		&excludePipeline,
		includeMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		includeMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		excludeMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		excludeMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, includeBacked.K8sObjects()...)
	resources = append(resources, excludeBackend.K8sObjects()...)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, includeBacked)
	assert.BackendReachable(t, excludeBackend)
	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.DeploymentReady(t, includeBacked.NamespacedName())
	assert.DeploymentReady(t, excludeBackend.NamespacedName())
	assert.MetricPipelineHealthy(t, includePipelineName)
	assert.MetricPipelineHealthy(t, excludePipelineName)

	assert.BackendDataEventuallyMatches(t, includeBacked,
		HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.NodeMetricsNames))),
	)
	assert.BackendDataEventuallyMatches(t, excludeBackend,
		HaveFlatMetrics(HaveUniqueNamesForRuntimeScope(ConsistOf(runtime.NodeMetricsNames))),
	)

	assert.BackendDataEventuallyMatches(t, includeBacked,
		HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsResourceAttributes))))),
	)
	assert.BackendDataEventuallyMatches(t, excludeBackend,
		HaveFlatMetrics(ContainElement(HaveResourceAttributes(HaveKeys(ConsistOf(runtime.NodeMetricsResourceAttributes))))),
	)
}
