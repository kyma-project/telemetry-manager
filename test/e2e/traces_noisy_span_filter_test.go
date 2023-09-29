//go:build e2e

package e2e

import (
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Filter Noisy Trace Spans", Label("tracing"), func() {

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
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should verify end-to-end trace delivery", func() {
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs)
		})

		It("Should filter noisy victoria metrics spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendVictoriaMetricsTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})

		It("Should filter noisy /metrics endpoint spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendMetricsEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})

		It("Should filter noisy /healthy endpoint spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendHealthzEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})

		It("Should filter noisy telemetry trace service push spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendTracePushServiceEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})

		It("Should filter noisy telemetry trace internal service spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendTraceInternalServiceEndpointTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})

		It("Should filter noisy fluent-bit service spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendFluentBitServiceTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})

		It("Should filter noisy metric gateway spans", func() {
			traceID, spanIDs, attrs, resAttrs := kittraces.MakeAndSendMetricGatewayEgressTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldNotBePresent(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs, resAttrs)
		})
	})
})
