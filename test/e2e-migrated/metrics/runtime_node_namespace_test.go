package metrics

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/metrics/runtime"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestRuntimeNodeNamespace(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetrics)

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
		WithOTLPOutput(testutils.OTLPEndpoint(includeBacked.Endpoint())).
		Build()
	excludePipeline := testutils.NewMetricPipelineBuilder().
		WithName(excludePipelineName).
		WithRuntimeInput(true, testutils.ExcludeNamespaces(excludeNs)).
		WithRuntimeInputNodeMetrics(true).
		WithRuntimeInputPodMetrics(false).
		WithRuntimeInputContainerMetrics(false).
		WithRuntimeInputVolumeMetrics(false).
		WithOTLPOutput(testutils.OTLPEndpoint(excludeBackend.Endpoint())).
		Build()

	includeMetricProducer := prommetricgen.New(includeNs)
	excludeMetricProducer := prommetricgen.New(excludeNs)

	resources := []client.Object{
		kitk8s.NewNamespace(includeNs).K8sObject(),
		kitk8s.NewNamespace(excludeNs).K8sObject(),
		kitk8s.NewNamespace(backendNs).K8sObject(),
		&includePipeline,
		&excludePipeline,
		includeMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		includeMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		excludeMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		excludeMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, includeBacked.K8sObjects()...)
	resources = append(resources, excludeBackend.K8sObjects()...)

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), resources...)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), resources...)).Should(Succeed())

	assert.DeploymentReady(t.Context(), kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t.Context(), kitkyma.MetricAgentName)
	assert.DeploymentReady(t.Context(), includeBacked.NamespacedName())
	assert.DeploymentReady(t.Context(), excludeBackend.NamespacedName())
	assert.MetricPipelineHealthy(t.Context(), includePipelineName)
	assert.MetricPipelineHealthy(t.Context(), excludePipelineName)

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
