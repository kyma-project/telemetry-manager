package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
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
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	istiofiedMetricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(istiofiedPipelineName).
		WithPrometheusInput(true).
		WithOTLPOutput(testutils.OTLPEndpoint(istiofiedBackend.EndpointHTTP())).
		Build()

	// generators to use the same ports as the backends => test istio communication
	generator := prommetricgen.New(backendNs, prommetricgen.WithMetricsPort(backend.Port()))
	istiofiedGenerator := prommetricgen.New(istiofiedBackendNs, prommetricgen.WithMetricsPort(istiofiedBackend.Port()))

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		kitk8sobjects.NewNamespace(istiofiedBackendNs, kitk8sobjects.WithIstioInjection()).K8sObject(),
		&metricPipeline,
		&istiofiedMetricPipeline,
		generator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).WithAvalancheLowLoad().K8sObject(),
		istiofiedGenerator.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTPS).WithAvalancheLowLoad().K8sObject(),
		generator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		istiofiedGenerator.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTPS).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)
	resources = append(resources, istiofiedBackend.K8sObjects()...)

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
