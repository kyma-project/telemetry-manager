//go:build istio

package istio

import (
	"context"
	"fmt"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"net/http"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Istio tracing", Label("tracing"), func() {

	const (
		mockNs             = "istio-tracing-filter-mock"
		mockDeploymentName = "istio-tracing-filter-backend"
		//creating mocks in a specially prepared namespace that allows calling workloads in the mesh via API server proxy
		sampleAppNs = "istio-permissive-mtls"
		//traceCollectorBaseName = "telemetry-trace-collector"
	)
	var (
		urls              = urlprovider.New()
		tracePipelineName = "pipeline-istio-traces"
		//traceCollectorName = types.NamespacedName{Name: traceCollectorBaseName, Namespace: kymaSystemNamespaceName}
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockDeploymentName, mockNs, backend.SignalTypeTraces)
		objs = append(objs, mockBackend.K8sObjects()...)

		urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
			mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
		)

		// Kyma-system namespace objects
		traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kymaSystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		urls.SetMetrics(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-traces-external", "metrics", ports.Metrics))
		objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-trace-collector")))
		urls.SetOTLPPush(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP))
		// Default namespace objects
		istioTracePipeline := kittrace.NewPipeline(tracePipelineName, mockBackend.HostSecretRefKey()).Persistent(true)

		objs = append(objs, istioTracePipeline.K8sObject())

		return objs
	}
	Context("App with istio-sidecar", Ordered, func() {

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Trace collector should answer requests", func() {
			By("Calling metrics service", func() {
				Eventually(func(g Gomega) {
					resp, err := proxyClient.Get(urls.Metrics())
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				}, timeout, interval).Should(Succeed())
			})
		})

		It("Should have istio-proxy spans in the backend", func() {
			By("Sending http requests", func() {
				for i := 0; i < 2; i++ {
					Eventually(func(g Gomega) {
						err := makeAndSendTraces(urls.OTLPPush())
						g.Expect(err).NotTo(HaveOccurred())
					}, timeout, interval).Should(Succeed())
				}
			})

			// Identify istio-proxy traces by component=proxy attribute
			proxyAttrs := pcommon.NewMap()
			proxyAttrs.PutStr("attrC", "vanilla")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithAttributes(proxyAttrs))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have custom spans in the backend", func() {
			// Identify sample app by serviceName attribute
			customResourceAttr := pcommon.NewMap()
			customResourceAttr.PutStr("service.name", "monitoring-custom-metrics")

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport(mockDeploymentName))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainSpansWithResourceAttributes(customResourceAttr))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeAndSendTraces(otlpPushURL string) error {
	traceID := kittraces.NewTraceID()
	var spanIDs []pcommon.SpanID

	spanIDs = append(spanIDs, kittraces.NewSpanID())

	attrs := pcommon.NewMap()
	attrs.PutStr("attrA", "chocolate")
	attrs.PutStr("attrB", "raspberry")
	attrs.PutStr("attrC", "vanilla")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("http.method", "GET")
	attrs.PutStr("OperationName", "some_component")
	traces := kittraces.MakeTraces(traceID, spanIDs, attrs)
	traces.ResourceSpans().At(0).Resource().Attributes()
	return sendTraces(context.Background(), traces, otlpPushURL)
}

func sendTraces(ctx context.Context, traces ptrace.Traces, otlpPushURL string) error {
	sender, err := kittraces.NewHTTPSender(ctx, otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}

	return sender.Export(ctx, traces)
}
