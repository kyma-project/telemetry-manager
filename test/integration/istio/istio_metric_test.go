//go:build istio

package istio

import (
	"net/http"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
		mockBackendName                  = "metric-agent-receiver"
		httpsAnnotatedMetricProducerName = "metric-producer-https"
		httpAnnotatedMetricProducerName  = "metric-producer-http"
		unannotatedMetricProducerName    = "metric-producer"
	)
	var (
		urls              = urlprovider.New()
		metricGatewayName = types.NamespacedName{Name: "telemetry-metric-gateway", Namespace: kymaSystemNamespaceName}
		metricAgentName   = types.NamespacedName{Name: "telemetry-metric-agent", Namespace: kymaSystemNamespaceName}
	)

	makeResources := func() ([]client.Object, *urlprovider.URLProvider) {
		var (
			objs         []client.Object
			grpcOTLPPort = 4317
			httpWebPort  = 80
		)

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// Mocks namespace objects.
		mockBackend := backend.New(mockBackendName, mockNs, "/metrics/"+telemetryDataFilename, backend.SignalTypeMetrics)
		mockBackendConfigMap := mockBackend.ConfigMap("metric-receiver-config")
		mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
		mockBackendExternalService := mockBackend.ExternalService().
			WithPort("grpc-otlp", grpcOTLPPort).
			WithPort("http-web", httpWebPort)

		httpsAnnotatedMetricProducer := metricproducer.New(mockNs, metricproducer.WithName(httpsAnnotatedMetricProducerName))
		httpAnnotatedMetricProducer := metricproducer.New(mockNs, metricproducer.WithName(httpAnnotatedMetricProducerName))
		unannotatedMetricProducer := metricproducer.New(mockNs, metricproducer.WithName(unannotatedMetricProducerName))

		// Default namespace objects.
		otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
		metricPipeline := kitmetric.NewPipeline("pipeline-with-prometheus-input-enabled", hostSecret.SecretKeyRef("metric-host")).PrometheusInput(true)

		objs = append(objs, []client.Object{
			mockBackendConfigMap.K8sObject(),
			mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			httpsAnnotatedMetricProducer.Pod().WithSidecarInjection().WithPrometheusAnnotations(metricproducer.SchemeHTTPS).K8sObject(),
			httpsAnnotatedMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTPS).K8sObject(),
			httpAnnotatedMetricProducer.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			httpAnnotatedMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			unannotatedMetricProducer.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			unannotatedMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
			hostSecret.K8sObject(),
			metricPipeline.K8sObject(),
		}...)

		urls.SetMockBackendExport(proxyClient.ProxyURLForService(mockNs, mockBackend.Name(), telemetryDataFilename, httpWebPort))

		return objs, urls
	}

	Context("App with istio-sidecar", Ordered, func() {
		BeforeAll(func() {
			k8sObjects, urlProvider := makeResources()
			urls = urlProvider

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
				key := types.NamespacedName{Name: mockBackendName, Namespace: mockNs}
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
				podScrapedMetricsShouldBeDelivered(urls, httpsAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme=http", func() {
				podScrapedMetricsShouldBeDelivered(urls, httpAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme unset", func() {
				podScrapedMetricsShouldBeDelivered(urls, unannotatedMetricProducerName)
			})
		})

		// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
		// targets discovered via annotated service must have the service label
		Context("Annotated services", func() {
			It("Should scrape if prometheus.io/scheme=https", func() {
				serviceScrapedMetricsShouldBeDelivered(urls, httpsAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme=http", func() {
				serviceScrapedMetricsShouldBeDelivered(urls, httpAnnotatedMetricProducerName)
			})

			It("Should scrape if prometheus.io/scheme unset", func() {
				serviceScrapedMetricsShouldBeDelivered(urls, unannotatedMetricProducerName)
			})
		})
	})
})

func podScrapedMetricsShouldBeDelivered(urls *urlprovider.URLProvider, podName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(urls.MockBackendExport())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainMd(SatisfyAll(
			WithMetrics(ContainElement(WithName(BeElementOf(metricproducer.AllMetricNames)))),
			WithResourceAttrs(ContainElement(HaveKeyWithValue("k8s.pod.name", podName))),
		))))
	}, timeout, telemetryDeliveryInterval).Should(Succeed())
}

func serviceScrapedMetricsShouldBeDelivered(urls *urlprovider.URLProvider, serviceName string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(urls.MockBackendExport())
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainMd(
			WithMetrics(ContainElement(SatisfyAll(
				WithDataPointAttrs(ContainElement(HaveKeyWithValue("service", serviceName))),
				WithName(BeElementOf(metricproducer.AllMetricNames)),
			))))))
	}, timeout, telemetryDeliveryInterval).Should(Succeed())
}
