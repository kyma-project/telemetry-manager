//go:build istio

package istio

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/prommetricgen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Prometheus Input", Label("metrics"), func() {
	const (
		mockNs                           = "istio-metric-prometheus-input"
		mockBackendName                  = "metric-agent-receiver"
		httpsAnnotatedMetricProducerName = "metric-producer-https"
		httpAnnotatedMetricProducerName  = "metric-producer-http"
		unannotatedMetricProducerName    = "metric-producer"
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// Mocks namespace objects
		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

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

		metricPipeline := kitk8s.NewMetricPipeline("pipeline-with-prometheus-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			PrometheusInput(true)
		objs = append(objs, metricPipeline.K8sObject())

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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		Context("Verify metric scraping works with annotating pods and services", Ordered, func() {
			// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
			// targets discovered via annotated pods must have no service label
			Context("Annotated pods", func() {
				It("Should scrape if prometheus.io/scheme=https", func() {
					podScrapedMetricsShouldBeDelivered(telemetryExportURL, httpsAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme=http", func() {
					podScrapedMetricsShouldBeDelivered(telemetryExportURL, httpAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme unset", func() {
					podScrapedMetricsShouldBeDelivered(telemetryExportURL, unannotatedMetricProducerName)
				})
			})

			// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
			// targets discovered via annotated service must have the service label
			Context("Annotated services", func() {
				It("Should scrape if prometheus.io/scheme=https", func() {
					serviceScrapedMetricsShouldBeDelivered(telemetryExportURL, httpsAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme=http", func() {
					serviceScrapedMetricsShouldBeDelivered(telemetryExportURL, httpAnnotatedMetricProducerName)
				})

				It("Should scrape if prometheus.io/scheme unset", func() {
					serviceScrapedMetricsShouldBeDelivered(telemetryExportURL, unannotatedMetricProducerName)
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
		g.Expect(resp).To(HaveHTTPBody(ContainMd(SatisfyAll(
			ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", podName)),
			ContainMetric(WithName(BeElementOf(prommetricgen.MetricNames))),
		))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func serviceScrapedMetricsShouldBeDelivered(proxyURL, serviceName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainMd(
			ContainMetric(SatisfyAll(
				WithName(BeElementOf(prommetricgen.MetricNames)),
				ContainDataPointAttrs(HaveKeyWithValue("service", serviceName)),
			)))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
