//go:build istio

package istio

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma/istio"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

var _ = Describe("Istio Metrics", Label("metrics"), func() {
	const (
		mockNs                           = "non-istio-metric-mock"
		mockBackendName                  = "metric-agent-receiver"
		httpsAnnotatedMetricProducerName = "metric-producer-https"
		httpAnnotatedMetricProducerName  = "metric-producer-http"
		unannotatedMetricProducerName    = "metric-producer"
		mockIstioBackendNs               = "istio-metric-mock"
		mockIstioBackendName             = "istiofied-metric-agent-receiver"
	)
	var telemetryExportURL, telemetryIstiofiedExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(mockIstioBackendNs, kitk8s.WithIstioInjection()).K8sObject())

		// Mocks namespace objects
		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		mockIstiofiedBackend := backend.New(mockIstioBackendName, mockIstioBackendNs, backend.SignalTypeMetrics, backend.ExcludeAPIAccessPort())
		objs = append(objs, mockIstiofiedBackend.K8sObjects()...)
		telemetryIstiofiedExportURL = mockIstiofiedBackend.TelemetryExportURL(proxyClient)
		fmt.Printf("UR: %v", telemetryIstiofiedExportURL)

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
		metricPipeline := kitmetric.NewPipeline("pipeline-with-prometheus-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			PrometheusInput(true)
		objs = append(objs, metricPipeline.K8sObject())

		metricPipelineIstiofiedBackend := kitmetric.NewPipeline("pipeline-with-istiofied-backend").
			WithOutputEndpointFromSecret(mockIstiofiedBackend.HostSecretRef()).
			PrometheusInput(true)

		objs = append(objs, metricPipelineIstiofiedBackend.K8sObject())
		// set peerauthentication to strict explicitly
		peerAuth := istio.NewPeerAuthentication(mockBackendName, mockIstioBackendNs)
		objs = append(objs, peerAuth.K8sObject(kitk8s.WithLabel("app", mockBackendName)))

		return objs
	}

	// We have 2 scenarious here:
	// 1. app(with Istio Sidecar)->metrics-agent->metric-gateway->non-istiofied-workload
	// 2. app(with Istio Sidecar)->metrics-agent->metric-gateway->istiofied-workload
	Context("With Istiofied and non-istiofied backend", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})
		Context("App with istio-sidecar and non istiofied backend", Ordered, func() {
			It("Should have a running metric gateway deployment", func() {
				verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
			})

			It("Should have a metrics backend running", func() {
				verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
			})

			It("Should have a running metric agent daemonset", func() {
				verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
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
		Context("App with istio-sidecar and istiofied backend", Ordered, func() {
			It("Should scrape if prometheus.io/scheme=https", func() {
				podScrapedMetricsShouldBeDelivered(telemetryIstiofiedExportURL, httpsAnnotatedMetricProducerName)
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
	}, time.Second*60, periodic.TelemetryInterval).Should(Succeed())
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
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
