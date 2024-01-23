//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/kubeletstats"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Runtime Input", Label("metrics"), func() {
	const (
		mockNs          = "metric-runtime-input-mocks"
		mockBackendName = "metric-agent-receiver"
	)

	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		metricPipeline := kitk8s.NewMetricPipeline("pipeline-with-runtime-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			RuntimeInput(true)
		pipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())

		return objs
	}

	Context("When a metricpipeline with Runtime input exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Ensures the metric gateway deployment is ready", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metrics backend is ready", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Ensures the metric agent daemonset is ready", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Ensures accessibility of metric agent metrics endpoint", Label(operationalTest), func() {
			agentMetricsURL := proxyClient.ProxyURLForService(kitkyma.MetricAgentMetrics.Namespace, kitkyma.MetricAgentMetrics.Name, "metrics", ports.Metrics)
			verifiers.ShouldExposeCollectorMetrics(proxyClient, agentMetricsURL)
		})

		It("Ensures the metric agent netpolicy exists", Label(operationalTest), func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(k8sClient.Get(ctx, kitkyma.MetricAgentNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.MetricAgentBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				metricAgentPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, metricAgentPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Ensures the metricpipeline is running", func() {
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Ensures kubeletstats metrics are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ContainMd(ContainMetric(WithName(BeElementOf(kubeletstats.MetricNames)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Checks for correct attributes in kubeletstats metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ConsistOfMds(ContainResourceAttrs(HaveKey(BeElementOf(kubeletstats.MetricResourceAttributes)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures kubeletstats metrics from system namespaces are not sent to backend", func() {
			verifiers.MetricsFromNamespaceShouldNotBeDelivered(proxyClient, telemetryExportURL, kitkyma.SystemNamespaceName)
		})
	})
})
