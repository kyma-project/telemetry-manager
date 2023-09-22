//go:build e2e

package e2e

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces", Label("tracing"), func() {
	const (
		mockBackendName = "traces-receiver"
		mockNs          = "traces-mocks"
	)

	var (
		pipelineName string
		urls         = urlprovider.New()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces, backend.WithPersistentHostSecret(true))
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		pipeline := kittrace.NewPipeline(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline"),
			mockBackend.HostSecretRefKey()).Persistent(true)
		pipelineName = pipeline.Name()
		objs = append(objs, pipeline.K8sObject())

		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kitkyma.KymaSystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
		)

		traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kitkyma.KymaSystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		urls.SetMetrics(proxyClient.ProxyURLForService(
			kitkyma.KymaSystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics))

		objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", kitkyma.TraceGatewayBaseName)))
		return objs
	}

	Context("When a tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", Label(operationalTest), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)

		})

		It("Should have 2 trace gateway replicas", Label(operationalTest), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.TraceGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(2)))
		})

		It("Should have a trace backend running", Label(operationalTest), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should be able to get trace gateway metrics endpoint", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.Metrics())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running pipeline", Label(operationalTest), func() {
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should verify end-to-end trace delivery", Label(operationalTest), func() {
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs)
		})

		It("Should have a working network policy", Label(operationalTest), func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(k8sClient.Get(ctx, kitkyma.TraceGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kitkyma.KymaSystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.TraceGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				traceGatewayPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kitkyma.KymaSystemNamespaceName, traceGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, timeout, interval).Should(Succeed())
		})
	})
})
