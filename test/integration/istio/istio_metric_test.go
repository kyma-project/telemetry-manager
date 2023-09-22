//go:build istio

package istio

import (
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
)

var _ = Describe("Istio metrics", Label("metrics"), func() {
	const (
		mockNs                           = "istio-metric-mock"
		mockDeploymentName               = "metric-agent-receiver"
		httpsAnnotatedMetricProducerName = "metric-producer-https"
		httpAnnotatedMetricProducerName  = "metric-producer-http"
		unannotatedMetricProducerName    = "metric-producer"
	)
	var (
		urls              = urlprovider.New()
		metricGatewayName = types.NamespacedName{Name: "telemetry-metric-gateway", Namespace: kymaSystemNamespaceName}
		metricAgentName   = types.NamespacedName{Name: "telemetry-metric-agent", Namespace: kymaSystemNamespaceName}
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// Mocks namespace objects
		mockBackend := backend.New(mockDeploymentName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
			mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
		)

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
			Eventually(func(g Gomega) {
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, metricGatewayName)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a metrics backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running metric agent daemonset", func() {
			Eventually(func(g Gomega) {
				ready, err := verifiers.IsDaemonSetReady(ctx, k8sClient, metricAgentName)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
		// targets discovered via annotated pods must have no service label
		Context("Annotated pods", func() {
			It("Should scrape if prometheus.io/scheme=https", func() {
				podScrapedMetricsShouldBeDelivered(urls.MockBackendExport(mockDeploymentName), httpsAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme=http", func() {
				podScrapedMetricsShouldBeDelivered(urls.MockBackendExport(mockDeploymentName), httpAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme unset", func() {
				podScrapedMetricsShouldBeDelivered(urls.MockBackendExport(mockDeploymentName), unannotatedMetricProducerName)
			})
		})

		// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
		// targets discovered via annotated service must have the service label
		Context("Annotated services", func() {
			It("Should scrape if prometheus.io/scheme=https", func() {
				serviceScrapedMetricsShouldBeDelivered(urls.MockBackendExport(mockDeploymentName), httpsAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme=http", func() {
				serviceScrapedMetricsShouldBeDelivered(urls.MockBackendExport(mockDeploymentName), httpAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme unset", func() {
				serviceScrapedMetricsShouldBeDelivered(urls.MockBackendExport(mockDeploymentName), unannotatedMetricProducerName)
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
