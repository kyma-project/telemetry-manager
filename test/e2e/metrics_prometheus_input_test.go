//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
)

var _ = Describe("Metrics Prometheus Input", Label("metrics"), func() {
	Context("When a metricpipeline exists", Ordered, func() {
		var (
			pipelines          *kyma.PipelineList
			urls               *urlprovider.URLProvider
			mockDeploymentName = "metric-agent-receiver"
			mockNs             = "metric-prometheus-input"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsPrometheusInputTestK8sObjects(mockNs, mockDeploymentName)
			pipelines = pipelinesProvider
			urls = urlProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			deploymentShouldBeReady(metricGatewayBaseName, kymaSystemNamespaceName)
		})

		It("Should have a metrics backend running", func() {
			deploymentShouldBeReady(mockDeploymentName, mockNs)

		})

		It("Should have a running metric agent daemonset", func() {
			daemonsetShouldBeReady(metricAgentBaseName, kymaSystemNamespaceName)
		})

		It("Should have a running pipeline", func() {
			metricPipelineShouldBeRunning(pipelines.First())
		})

		It("Should verify custom metric scraping via annotated pods", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
				// targets discovered via annotated pods must have no service label
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
					))),
				),
				))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify custom metric scraping via annotated services", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
						ContainDataPointAttrs(HaveKey("service")),
					))),
				),
				))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify no kubelet metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainMd(ContainMetric(WithName(BeElementOf(kubeletMetricNames))))),
				))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeMetricsPrometheusInputTestK8sObjects(mockNs string, mockDeploymentName string) ([]client.Object, *urlprovider.URLProvider, *kyma.PipelineList) {
	var (
		objs      []client.Object
		pipelines = kyma.NewPipelineList()
		urls      = urlprovider.New()
	)

	objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

	// Mocks namespace objects.
	mockBackend, err := backend.New(mockDeploymentName, mockNs, backend.SignalTypeMetrics)
	Expect(err).NotTo(HaveOccurred())

	mockMetricProducer := metricproducer.New(mockNs)
	objs = append(objs, mockBackend.K8sObjects()...)
	objs = append(objs, []client.Object{
		mockMetricProducer.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		mockMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
	}...)
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	metricPipeline := kitmetric.NewPipeline("pipeline-with-prometheus-input-enabled", mockBackend.HostSecretRefKey()).PrometheusInput(true)
	pipelines.Append(metricPipeline.Name())
	objs = append(objs, metricPipeline.K8sObject())

	return objs, urls, pipelines
}
