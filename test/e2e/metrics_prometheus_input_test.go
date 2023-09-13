//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
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
			mocksNs            = "metric-prometheus-input"
			metricGatewayName  = types.NamespacedName{Name: metricAgentGatewayBaseName, Namespace: kymaSystemNamespaceName}
			metricAgentName    = types.NamespacedName{Name: metricAgentBaseName, Namespace: kymaSystemNamespaceName}
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsPrometheusInputTestK8sObjects(mocksNs, mockDeploymentName)
			pipelines = pipelinesProvider
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
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mocksNs}
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

		It("Should have a running pipeline", func() {
			metricPipelineShouldBeRunning(pipelines.First())
		})

		It("Should verify custom metric scraping via annotated pods", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
				// targets discovered via annotated pods must have no service label
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
					)))),
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
					)))),
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
					)))),
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
					)))),
				),
				))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify custom metric scraping via annotated services", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
						WithDataPointAttrs(ContainElement(HaveKey("service"))),
					)))),
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
						WithDataPointAttrs(ContainElement(HaveKey("service"))),
					)))),
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
						WithDataPointAttrs(ContainElement(HaveKey("service"))),
					)))),
					ContainMd(WithMetrics(ContainElement(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
						WithDataPointAttrs(ContainElement(HaveKey("service"))),
					)))),
				),
				))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify no kubelet metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					Not(ContainMd(WithMetrics(ContainElement(WithName(BeElementOf(kubeletMetricNames)))))),
				))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeMetricsPrometheusInputTestK8sObjects(mocksNamespaceName string, mockDeploymentName string) ([]client.Object, *urlprovider.URLProvider, *kyma.PipelineList) {
	var (
		objs         []client.Object
		pipelines    = kyma.NewPipelineList()
		urls         = urlprovider.New()
		grpcOTLPPort = 4317
		httpWebPort  = 80
	)

	mocksNamespace := kitk8s.NewNamespace(mocksNamespaceName)
	objs = append(objs, kitk8s.NewNamespace(mocksNamespaceName).K8sObject())

	// Mocks namespace objects.
	mockBackend := backend.New(mockDeploymentName, mocksNamespace.Name(), "/metrics/"+telemetryDataFilename, backend.SignalTypeMetrics)
	mockBackendConfigMap := mockBackend.ConfigMap("metric-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-web", httpWebPort)
	mockMetricProducer := metricproducer.New(mocksNamespaceName)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline := kitmetric.NewPipeline("pipeline-with-prometheus-input-enabled", hostSecret.SecretKeyRef("metric-host")).PrometheusInput(true)
	pipelines.Append(metricPipeline.Name())

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockMetricProducer.Pod().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		mockMetricProducer.Service().WithPrometheusAnnotations(metricproducer.SchemeHTTP).K8sObject(),
		hostSecret.K8sObject(),
		metricPipeline.K8sObject(),
	}...)

	urls.SetMockBackendExport(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort))

	return objs, urls, pipelines
}
