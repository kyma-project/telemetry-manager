package istio

import (
	"io"
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
	backendExportURL := backend.ExportURL(suite.ProxyClient)

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

	podMetricsShouldNotBeDelivered := func(proxyURL, podName string) {
		Consistently(func(g Gomega) {
			resp, err := suite.ProxyClient.Get(proxyURL)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp).To(HaveHTTPStatus(200))
			bodyContent, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(bodyContent).To(HaveFlatMetrics(
				Not(ContainElement(SatisfyAll(
					HaveName(BeElementOf(prommetricgen.CustomMetricNames())),
					Not(HaveMetricAttributes(HaveKey("service"))),
					HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", podName)),
				))),
			))
		}, 3*periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
	}

	podScrapedMetricsShouldBeDelivered := func(proxyURL, podName string) {
		Eventually(func(g Gomega) {
			resp, err := suite.ProxyClient.Get(proxyURL)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp).To(HaveHTTPStatus(200))
			bodyContent, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(bodyContent).To(HaveFlatMetrics(
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
			))
		}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
	}

	serviceScrapedMetricsShouldBeDelivered := func(proxyURL, serviceName string) {
		Eventually(func(g Gomega) {
			resp, err := suite.ProxyClient.Get(proxyURL)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp).To(HaveHTTPStatus(200))
			bodyContent, err := io.ReadAll(resp.Body)
			defer resp.Body.Close()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(bodyContent).To(HaveFlatMetrics(
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
		}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
	}

	podMetricsShouldNotBeDelivered(backendExportURL, httpsAnnotatedMetricProducerName)
	podScrapedMetricsShouldBeDelivered(backendExportURL, httpAnnotatedMetricProducerName)
	podScrapedMetricsShouldBeDelivered(backendExportURL, unannotatedMetricProducerName)
	serviceScrapedMetricsShouldBeDelivered(backendExportURL, httpsAnnotatedMetricProducerName)
	serviceScrapedMetricsShouldBeDelivered(backendExportURL, httpAnnotatedMetricProducerName)
	serviceScrapedMetricsShouldBeDelivered(backendExportURL, unannotatedMetricProducerName)
}
