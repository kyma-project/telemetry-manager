//go:build e2e

package e2e

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s/verifiers"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
)

var (
	metricAgentGatewayBaseName = "telemetry-metric-gateway"
	metricAgentBaseName        = "telemetry-metric-agent"

	kubeletMetricAttributes = []string{"k8s.cluster.name", "k8s.container.name", "k8s.daemonset.name", "k8s.deployment.name", "k8s.namespace.name", "k8s.node.name", "k8s.pod.name", "k8s.pod.uid", "kyma.source"}
	kubeletMetricNames      = []string{"container.cpu.time", "container.cpu.utilization", "container.filesystem.available", "container.filesystem.capacity", "container.filesystem.usage", "container.memory.available", "container.memory.major_page_faults", "container.memory.page_faults", "container.memory.rss", "container.memory.usage", "container.memory.working_set", "k8s.pod.cpu.time", "k8s.pod.cpu.utilization", "k8s.pod.filesystem.available", "k8s.pod.filesystem.capacity", "k8s.pod.filesystem.usage", "k8s.pod.memory.available", "k8s.pod.memory.major_page_faults", "k8s.pod.memory.page_faults", "k8s.pod.memory.rss", "k8s.pod.memory.usage", "k8s.pod.memory.working_set", "k8s.pod.network.errors", "k8s.pod.network.io"}
)

var _ = Describe("Metrics Kubeletstats", Label("metrics"), func() {
	Context("When a metricpipeline exists", Ordered, func() {
		var (
			pipelines          *kyma.PipelineList
			urls               *mocks.URLProvider
			mockDeploymentName = "metric-agent-receiver"
			mockNs             = "metric-agent-mocks"
			metricGatewayName  = types.NamespacedName{Name: metricAgentGatewayBaseName, Namespace: kymaSystemNamespaceName}
			metricAgentName    = types.NamespacedName{Name: metricAgentBaseName, Namespace: kymaSystemNamespaceName}
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsAgentTestK8sObjects(mockNs, mockDeploymentName)
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

		It("Should have a running pipeline", func() {
			metricPipelineShouldBeRunning(pipelines.First())
		})

		It("Should verify kubelet metric names", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					HaveMetricNames(kubeletMetricNames...))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify kubelet metric attributes", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					HaveAttributes(kubeletMetricAttributes...))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

// makeMetricsAgentTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeMetricsAgentTestK8sObjects(namespace string, mockDeploymentNames ...string) ([]client.Object, *mocks.URLProvider, *kyma.PipelineList) {
	var (
		objs         []client.Object
		pipelines    = kyma.NewPipelineList()
		urls         = mocks.NewURLProvider()
		grpcOTLPPort = 4317
		httpWebPort  = 80
	)

	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, mocksNamespace.K8sObject())

	for i, mockDeploymentName := range mockDeploymentNames {
		// Mocks namespace objects.
		mockBackend := mocks.NewBackend(suffixize(mockDeploymentName, i), mocksNamespace.Name(), "/metrics/"+telemetryDataFilename, mocks.SignalTypeMetrics)
		mockBackendConfigMap := mockBackend.ConfigMap(suffixize("metric-receiver-config", i))
		mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
		mockBackendExternalService := mockBackend.ExternalService().
			WithPort("grpc-otlp", grpcOTLPPort).
			WithPort("http-web", httpWebPort)

		// Default namespace objects.
		otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL)).Persistent(isOperational())
		metricPipeline := kitmetric.NewPipeline(fmt.Sprintf("%s-%s", mockDeploymentName, "pipeline"), hostSecret.SecretKeyRef("metric-host")).RuntimeInput(true)
		pipelines.Append(metricPipeline.Name())

		objs = append(objs, []client.Object{
			mockBackendConfigMap.K8sObject(),
			mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			hostSecret.K8sObject(),
			metricPipeline.K8sObject(),
		}...)

		urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), i)
	}

	return objs, urls, pipelines
}
