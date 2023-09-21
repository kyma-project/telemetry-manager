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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
)

var (
	metricAgentBaseName = "telemetry-metric-agent"

	kubeletMetricAttributes = []string{"k8s.cluster.name", "k8s.container.name", "k8s.namespace.name", "k8s.node.name", "k8s.pod.name", "k8s.pod.uid"}
	kubeletMetricNames      = []string{"container.cpu.time", "container.cpu.utilization", "container.filesystem.available", "container.filesystem.capacity", "container.filesystem.usage", "container.memory.available", "container.memory.major_page_faults", "container.memory.page_faults", "container.memory.rss", "container.memory.usage", "container.memory.working_set", "k8s.pod.cpu.time", "k8s.pod.cpu.utilization", "k8s.pod.filesystem.available", "k8s.pod.filesystem.capacity", "k8s.pod.filesystem.usage", "k8s.pod.memory.available", "k8s.pod.memory.major_page_faults", "k8s.pod.memory.page_faults", "k8s.pod.memory.rss", "k8s.pod.memory.usage", "k8s.pod.memory.working_set", "k8s.pod.network.errors", "k8s.pod.network.io"}
)

var _ = Describe("Metrics Runtime Input", Label("metrics"), func() {
	Context("When a metricpipeline exists", Ordered, func() {
		var (
			pipelines          *kyma.PipelineList
			urls               *urlprovider.URLProvider
			mockDeploymentName = "metric-agent-receiver"
			mockNs             = "metric-runtime-input-mocks"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsRuntimeInputTestK8sObjects(mockNs, mockDeploymentName)
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

		It("Should verify kubelet metric names", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(ContainMetric(WithName(BeElementOf(kubeletMetricNames)))),
				))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify kubelet metric attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ConsistOfMds(ContainResourceAttrs(HaveKey(BeElementOf(kubeletMetricAttributes)))),
				))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeMetricsRuntimeInputTestK8sObjects(mocksNamespaceName string, mockDeploymentName string) ([]client.Object, *urlprovider.URLProvider, *kyma.PipelineList) {
	var (
		objs      []client.Object
		pipelines = kyma.NewPipelineList()
		urls      = urlprovider.New()
	)

	mocksNamespace := kitk8s.NewNamespace(mocksNamespaceName)
	objs = append(objs, kitk8s.NewNamespace(mocksNamespaceName).K8sObject())

	// Mocks namespace objects.
	mockBackend := backend.New(mocksNamespace.Name(), mockDeploymentName, backend.SignalTypeMetrics).Build()
	objs = append(objs, mockBackend.K8sObjects()...)
	urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
		mocksNamespaceName, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
	)

	// Default namespace objects.
	metricPipeline := kitmetric.NewPipeline("pipeline-with-runtime-input-enabled", mockBackend.GetHostSecretRefKey()).RuntimeInput(true)
	pipelines.Append(metricPipeline.Name())
	objs = append(objs, metricPipeline.K8sObject())

	return objs, urls, pipelines
}
