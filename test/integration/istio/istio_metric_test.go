//go:build istio

package istio

import (
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
)

var _ = Describe("Istio metrics", Label("metrics"), func() {
	const (
		mockNs                           = "istio-metric-mock"
		mockBackendName                  = "metric-agent-receiver"
		httpsAnnotatedMetricProducerName = "metric-producer-https"
		httpAnnotatedMetricProducerName  = "metric-producer-http"
		unannotatedMetricProducerName    = "metric-producer"
	)
	var (
		telemetryExportURL string
		metricGatewayName  = types.NamespacedName{Name: "telemetry-metric-gateway", Namespace: kymaSystemNamespaceName}
		metricAgentName    = types.NamespacedName{Name: "telemetry-metric-agent", Namespace: kymaSystemNamespaceName}
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// Mocks namespace objects
		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		httpsAnnotatedMetricProducer := metricproducer.New(mockNs, metricproducer.WithName(httpsAnnotatedMetricProducerName))
		httpAnnotatedMetricProducer := metricproducer.New(mockNs, metricproducer.WithName(httpAnnotatedMetricProducerName))
		unannotatedMetricProducer := metricproducer.New(mockNs, metricproducer.WithName(unannotatedMetricProducerName))
		objs = append(objs, []client.Object{
			httpsAnnotatedMetricProducer.Pod().WithSidecarInjection().WithPrometheusAnnotations(metricproducer.SchemeHTTPS).K8sObject(),
			httpsAnnotatedMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTPS).K8sObject(),
			httpAnnotatedMetricProducer.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			httpAnnotatedMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			unannotatedMetricProducer.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			unannotatedMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		}...)

		// Default namespace objects
		metricPipeline := kitmetric.NewPipeline("pipeline-with-prometheus-input-enabled", mockBackend.HostSecretRefKey()).
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, metricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, metricAgentName)
		})

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

func podScrapedMetricsShouldBeDelivered(proxyURL, podName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainMd(SatisfyAll(
			ContainResourceAttrs(HaveKeyWithValue("k8s.pod.name", podName)),
			ContainMetric(WithName(BeElementOf(metricproducer.AllMetricNames))),
		))))
	}, timeout, telemetryDeliveryInterval).Should(Succeed())
}

func serviceScrapedMetricsShouldBeDelivered(proxyURL, serviceName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainMd(
			ContainMetric(SatisfyAll(
				WithName(BeElementOf(metricproducer.AllMetricNames)),
				ContainDataPointAttrs(HaveKeyWithValue("service", serviceName)),
			)))))
	}, timeout, telemetryDeliveryInterval).Should(Succeed())
}
