package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsPrometheusInput(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelIstio)

	var (
		uniquePrefix                     = unique.Prefix()
		pipelineName                     = uniquePrefix()
		backendNs                        = uniquePrefix("backend")
		genNs                            = uniquePrefix("gen")
		httpsAnnotatedMetricProducerName = uniquePrefix("producer-https")
		httpAnnotatedMetricProducerName  = uniquePrefix("producer-http")
		unannotatedMetricProducerName    = uniquePrefix("producer")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeMetrics)

	httpsAnnotatedMetricProducer := prommetricgen.New(genNs, prommetricgen.WithName(httpsAnnotatedMetricProducerName))
	httpAnnotatedMetricProducer := prommetricgen.New(genNs, prommetricgen.WithName(httpAnnotatedMetricProducerName))
	unannotatedMetricProducer := prommetricgen.New(genNs, prommetricgen.WithName(unannotatedMetricProducerName))

	metricPipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithPrometheusInput(true, testutils.IncludeNamespaces(genNs)).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
		Build()

	resources := []client.Object{
		kitk8s.NewNamespace(backendNs).K8sObject(),
		kitk8s.NewNamespace(genNs).K8sObject(),
		&metricPipeline,
		httpsAnnotatedMetricProducer.Pod().WithSidecarInjection().WithPrometheusAnnotations(prommetricgen.SchemeHTTPS).K8sObject(),
		httpsAnnotatedMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTPS).K8sObject(),
		httpAnnotatedMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		httpAnnotatedMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		unannotatedMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeNone).K8sObject(),
		unannotatedMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeNone).K8sObject(),
	}
	resources = append(resources, backend.K8sObjects()...)

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.DeploymentReady(t, kitkyma.MetricGatewayName)
	assert.DaemonSetReady(t, kitkyma.MetricAgentName)
	assert.BackendReachable(t, backend)

	podMetricsShouldNotBeDelivered(t, backend, httpsAnnotatedMetricProducerName)
	podScrapedMetricsShouldBeDelivered(t, backend, httpAnnotatedMetricProducerName)
	podScrapedMetricsShouldBeDelivered(t, backend, unannotatedMetricProducerName)
	serviceScrapedMetricsShouldBeDelivered(t, backend, httpsAnnotatedMetricProducerName)
	serviceScrapedMetricsShouldBeDelivered(t, backend, httpAnnotatedMetricProducerName)
	serviceScrapedMetricsShouldBeDelivered(t, backend, unannotatedMetricProducerName)
}

func podMetricsShouldNotBeDelivered(t *testing.T, backend *kitbackend.Backend, podName string) {
	assert.BackendDataConsistentlyMatches(t, backend, HaveFlatMetrics(
		Not(ContainElement(SatisfyAll(
			HaveName(BeElementOf(prommetricgen.CustomMetricNames())),
			Not(HaveMetricAttributes(HaveKey("service"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", podName)),
		)))), assert.WithCustomTimeout(3*periodic.TelemetryConsistentlyTimeout),
	)
}

func podScrapedMetricsShouldBeDelivered(t *testing.T, backend *kitbackend.Backend, podName string) {
	assert.BackendDataConsistentlyMatches(t, backend, HaveFlatMetrics(
		ContainElement(SatisfyAll(
			HaveName(BeElementOf(prommetricgen.CustomMetricNames())),
			Not(HaveMetricAttributes(HaveKey("service"))),
			HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", podName)),
			HaveScopeName(Equal(InstrumentationScopePrometheus)),
			HaveScopeVersion(SatisfyAny(
				Equal("main"),
				MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
			)),
		)),
	),
	)
}

func serviceScrapedMetricsShouldBeDelivered(t *testing.T, backend *kitbackend.Backend, serviceName string) {
	assert.BackendDataEventuallyMatches(t, backend, HaveFlatMetrics(
		ContainElement(SatisfyAll(
			HaveName(BeElementOf(prommetricgen.CustomMetricNames())),
			HaveMetricAttributes(HaveKeyWithValue("service", serviceName)),
			HaveScopeName(Equal(InstrumentationScopePrometheus)),
			HaveScopeVersion(SatisfyAny(
				Equal("main"),
				MatchRegexp("[0-9]+.[0-9]+.[0-9]+"),
			)),
		)),
	))
}
