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

	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/otel/kubeletstats"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName).
			WithOutputEndpointFromSecret(backend.HostSecretRefV1Alpha1()).
			RuntimeInput(true)
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
			assert.DeploymentReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Ensures the metrics backend is ready", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Ensures the metric agent daemonset is ready", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Ensures accessibility of metric agent metrics endpoint", func() {
			agentMetricsURL := proxyClient.ProxyURLForService(kitkyma.MetricAgentMetrics.Namespace, kitkyma.MetricAgentMetrics.Name, "metrics", ports.Metrics)
			assert.ExposesOTelCollectorMetrics(proxyClient, agentMetricsURL)
		})

		It("Ensures the metric agent network policy exists", func() {
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
			assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Ensures kubeletstats metrics are sent to backend", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(ContainMd(SatisfyAll(
					ContainMetric(WithName(BeElementOf(kubeletstats.MetricNames))),
					ContainScope(WithScopeName(ContainSubstring(InstrumentationScopeRuntime))),
				))))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Checks for correct attributes in kubeletstats metrics", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(
					ConsistOfMds(ContainResourceAttrs(HaveKey(BeElementOf(kubeletstats.MetricResourceAttributes)))),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Ensures kubeletstats metrics from system namespaces are not sent to backend", func() {
			assert.MetricsFromNamespaceNotDelivered(proxyClient, backendExportURL, kitkyma.SystemNamespaceName)
		})
	})
})
