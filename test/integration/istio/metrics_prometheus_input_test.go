//go:build istio

package istio

import (
	"io"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelIntegration), Ordered, func() {
	var (
		mockNs                           = suite.ID()
		pipelineName                     = suite.ID()
		httpsAnnotatedMetricProducerName = suite.IDWithSuffix("producer-https")
		httpAnnotatedMetricProducerName  = suite.IDWithSuffix("producer-http")
		unannotatedMetricProducerName    = suite.IDWithSuffix("producer")
	)
	var backendExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// Mocks namespace objects
		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		httpsAnnotatedMetricProducer := prommetricgen.New(mockNs, prommetricgen.WithName(httpsAnnotatedMetricProducerName))
		httpAnnotatedMetricProducer := prommetricgen.New(mockNs, prommetricgen.WithName(httpAnnotatedMetricProducerName))
		unannotatedMetricProducer := prommetricgen.New(mockNs, prommetricgen.WithName(unannotatedMetricProducerName))
		objs = append(objs, []client.Object{
			httpsAnnotatedMetricProducer.Pod().WithSidecarInjection().WithPrometheusAnnotations(prommetricgen.SchemeHTTPS).K8sObject(),
			httpsAnnotatedMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTPS).K8sObject(),
			httpAnnotatedMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			httpAnnotatedMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			unannotatedMetricProducer.Pod().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
			unannotatedMetricProducer.Service().WithPrometheusAnnotations(prommetricgen.SchemeHTTP).K8sObject(),
		}...)

		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithPrometheusInput(true, testutils.IncludeNamespaces(mockNs)).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &metricPipeline)

		return objs
	}

	Context("App with istio-sidecar", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a running metric agent daemonset", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		Context("Verify metric scraping works with annotating pods and services", Ordered, func() {
			// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
			// targets discovered via annotated pods must have no service label
			Context("Annotated pods", func() {
				It("Should scrape if prometheus.io/scheme=https", func() {
					podScrapedMetricsShouldBeDelivered(backendExportURL, httpsAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme=http", func() {
					podScrapedMetricsShouldBeDelivered(backendExportURL, httpAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme unset", func() {
					podScrapedMetricsShouldBeDelivered(backendExportURL, unannotatedMetricProducerName)
				})
			})

			// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
			// targets discovered via annotated service must have the service label
			Context("Annotated services", func() {
				It("Should scrape if prometheus.io/scheme=https", func() {
					serviceScrapedMetricsShouldBeDelivered(backendExportURL, httpsAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme=http", func() {
					serviceScrapedMetricsShouldBeDelivered(backendExportURL, httpAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme unset", func() {
					serviceScrapedMetricsShouldBeDelivered(backendExportURL, unannotatedMetricProducerName)
				})
			})
		})
	})
})

func podScrapedMetricsShouldBeDelivered(proxyURL, podName string) {

	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(WithFlatMetricsDataPoints(
			SatisfyAll(
				ContainElement(WithResourceAttributes(HaveKeyWithValue("k8s.pod.name", podName))),
				ContainElement(WithName(BeElementOf(prommetricgen.MetricNames))),
				ContainElement(WithScopeName(ContainSubstring(InstrumentationScopePrometheus))),
			),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func serviceScrapedMetricsShouldBeDelivered(proxyURL, serviceName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

		bodyContent, err := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(bodyContent).To(WithFlatMetricsDataPoints(
			SatisfyAll(
				ContainElement(WithName(BeElementOf(prommetricgen.MetricNames))),
				ContainElement(WithMetricAttributes(HaveKeyWithValue("service", serviceName))),
			),
		))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
