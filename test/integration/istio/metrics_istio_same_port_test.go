package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsIstioSamePort(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelGardener, suite.LabelIstio)

	var (
		uniquePrefix          = unique.Prefix()
		pipelineName          = uniquePrefix("pipeline")
		istiofiedPipelineName = uniquePrefix("pipeline-istiofied")
		backendNs             = uniquePrefix("backend")
		istiofiedBackendNs    = uniquePrefix("backend-istiofied")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)
	istiofiedBackend := kitbackend.New(istiofiedBackendNs, kitbackend.SignalTypeMetrics)

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithPrometheusInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	istiofiedMetricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(istiofiedPipelineName).
		WithPrometheusInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(istiofiedBackend.Endpoint())).
		Build()

	// generators to use the same ports as the backends => test istio communication
	generator := prommetricgen.New(backendNs, prommetricgen.WithMetricsPort(backend.Port()))
	istiofiedGenerator := prommetricgen.New(istiofiedBackendNs, prommetricgen.WithMetricsPort(istiofiedBackend.Port()))

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(istiofiedBackendNs, kitk8s.WithIstioInjection()).K8sObject(),
		&metricPipeline,
		&istiofiedMetricPipeline,
		generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).WithAvalanche(1, 1, 1).K8sObject(),
		istiofiedGenerator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).WithAvalanche(1, 1, 1).K8sObject(),
		generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		istiofiedGenerator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, istiofiedBackend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())

		for _, resource := range resources {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: resource.GetName(), Namespace: resource.GetNamespace()}
				err := suite.K8sClient.Get(suite.Ctx, key, resource)
				g.Expect(err == nil).To(BeFalse())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		}
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, backend)
	assert.BackendReachable(t, istiofiedBackend)

	assert.MetricsFromNamespaceDelivered(t, backend, backendNs, prommetricgen.AvalancheMetricNames())
	assert.MetricsFromNamespaceDelivered(t, backend, istiofiedBackendNs, prommetricgen.AvalancheMetricNames())
	assert.MetricsFromNamespaceDelivered(t, istiofiedBackend, backendNs, prommetricgen.AvalancheMetricNames())
	assert.MetricsFromNamespaceDelivered(t, istiofiedBackend, istiofiedBackendNs, prommetricgen.AvalancheMetricNames())
}
