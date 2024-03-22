//go:build e2e

package e2e

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otel/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Telemetry Self Monitor", Label("self-mon"), Ordered, func() {
	const (
		mockBackendName = "traces-receiver-selfmon"
		mockNs          = "traces-basic-selfmon-test"
	)

	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces, backend.WithPersistentHostSecret(isOperational()))
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		pipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-pipeline", mockBackend.Name())).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1()).
			Persistent(isOperational())
		pipelineName = pipeline.Name()
		objs = append(objs, pipeline.K8sObject())

		return objs
	}

	Context("When a trace pipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})
		It("Should have a running self-monitor", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.SelfMonitorName)
		})
		It("Should have a network policy deployed", func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(k8sClient.Get(ctx, kitkyma.SelfMonitorNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.SelfMonitorBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				selfMonitorPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, selfMonitorPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have service deployed", func() {
			var service corev1.Service
			Expect(k8sClient.Get(ctx, kitkyma.SelfMonitorName, &service)).To(Succeed())
		})

		It("Should have a running pipeline", Label(operationalTest), func() {
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should verify end-to-end trace delivery", Label(operationalTest), func() {
			gatewayPushURL := proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP)
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, gatewayPushURL)
			verifiers.TracesShouldBeDelivered(proxyClient, telemetryExportURL, traceID, spanIDs, attrs)
		})

		It("Should be able to get trace gateway metrics endpoint", Label(operationalTest), func() {
			gatewayMetricsURL := proxyClient.ProxyURLForService(kitkyma.TraceGatewayMetrics.Namespace, kitkyma.TraceGatewayMetrics.Name, "metrics", ports.Metrics)
			verifiers.ShouldExposeCollectorMetrics(proxyClient, gatewayMetricsURL)
		})

		It("The telemetryFlowHealthy condition should be true", func() {
			verifiers.TracePipelineTelemetryHealthFlowIsHealthy(ctx, k8sClient, pipelineName)
		})
	})
})
