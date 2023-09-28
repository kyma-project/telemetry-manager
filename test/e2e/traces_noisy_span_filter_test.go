//go:build e2e

package e2e

import (
	"fmt"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Traces Noisy Span Filter", Label("tracing"), func() {

	const (
		mockBackendName = "traces-filter-receiver"
		mockNs          = "traces-filter-test"
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

		pipeline := kittrace.NewPipeline(fmt.Sprintf("%s-pipeline", mockBackend.Name())).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			Persistent(isOperational())
		pipelineName = pipeline.Name()
		objs = append(objs, pipeline.K8sObject())

		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
		)

		traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kitkyma.SystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		urls.SetMetrics(proxyClient.ProxyURLForService(
			kitkyma.SystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics))

		objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", kitkyma.TraceGatewayBaseName)))
		return objs
	}

	Context("When noisy span present", Ordered, func() {

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should be able to get trace gateway metrics endpoint", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.Metrics())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have a running pipeline", Label(operationalTest), func() {
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should verify end-to-end trace delivery", Label(operationalTest), func() {
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs)
		})

		It("Should filter noisy traces and spans", Label(operationalTest), func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendVictoriaMetricsTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)

			traceID, spanIDs, attrs, resAttrs = kittraces.MakeAndSendMetricsEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)

			traceID, spanIDs, attrs, resAttrs = kittraces.MakeAndSendHealthzEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)

			traceID, spanIDs, attrs, resAttrs = kittraces.MakeAndSendTracePushServiceEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)

			traceID, spanIDs, attrs, resAttrs = kittraces.MakeAndSendTraceInternalServiceEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)

			traceID, spanIDs, attrs, resAttrs = kittraces.MakeAndSendFluentBitServiceTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)

			traceID, spanIDs, attrs, resAttrs = kittraces.MakeAndSendMetricGatewayEgressTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})
	})
})
