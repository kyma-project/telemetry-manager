//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otel/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces Noisy Span Filter", Label("traces"), func() {

	const (
		mockBackendName = "traces-filter-receiver"
		mockNs          = "traces-noisy-span-filter-test"
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

		pipeline := kitk8s.NewTracePipeline(fmt.Sprintf("%s-pipeline", mockBackend.Name())).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
		pipelineName = pipeline.Name()
		objs = append(objs, pipeline.K8sObject())

		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
		)

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

		It("Should have a running pipeline", func() {
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a trace backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should verify end-to-end trace delivery", func() {
			traceID, spanIDs, spanAttrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, spanAttrs)
		})

		It("Should filter noisy victoria metrics spans", func() {
			traceID := kittraces.MakeAndSendVictoriaMetricsAgentTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy metric agent scrape spans", func() {
			traceID := kittraces.MakeAndSendMetricAgentScrapeTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy /healthy endpoint spans", func() {
			traceID := kittraces.MakeAndSendIstioHealthzEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy telemetry trace service spans", func() {
			traceID := kittraces.MakeAndSendTraceServiceTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy telemetry trace internal service spans", func() {
			traceID := kittraces.MakeAndSendTraceInternalServiceTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy telemetry metric service spans", func() {
			traceID := kittraces.MakeAndSendMetricServiceTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy fluent-bit spans", func() {
			traceID := kittraces.MakeAndSendFluentBitTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy metric gateway spans", func() {
			traceID := kittraces.MakeAndSendMetricGatewayTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy trace gateway spans", func() {
			traceID := kittraces.MakeAndSendTraceGatewayTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy metric gateway spans", func() {
			traceID := kittraces.MakeAndSendMetricGatewayTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})

		It("Should filter noisy metric agent spans", func() {
			traceID := kittraces.MakeAndSendMetricAgentTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID)
		})
	})
})
