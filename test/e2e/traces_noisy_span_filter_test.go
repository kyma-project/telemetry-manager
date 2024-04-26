//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces Noisy Span Filter", Label("traces"), func() {

	const (
		mockBackendName = "traces-filter-receiver"
		mockNs          = "traces-noisy-span-filter-test"

		// regular spans should NOT be filtered
		regularSpansNs = "regular-spans"

		// noisy spans should be filtered
		vmaScrapeSpansNs            = "vma-scrape-spans"
		healthzSpansNs              = "healthz-spans"
		fluentBitSpansNs            = "fluent-bit-spans"
		metricAgentScrapeSpansNs    = "metric-agent-scrape-spans"
		metricAgentSpansNs          = "metric-agent-spans"
		metricGatewaySpansNs        = "metric-gateway-spans"
		metricServiceSpansNs        = "metric-service-spans"
		traceGatewaySpansNs         = "trace-gateway-spans"
		traceServiceSpansNs         = "trace-service-spans"
		traceServiceInternalSpansNs = "trace-service-internal-spans"
	)

	var (
		pipelineName       string
		telemetryExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
			kitk8s.NewNamespace(regularSpansNs).K8sObject(),
			kitk8s.NewNamespace(vmaScrapeSpansNs).K8sObject(),
			kitk8s.NewNamespace(healthzSpansNs).K8sObject(),
			kitk8s.NewNamespace(fluentBitSpansNs).K8sObject(),
			kitk8s.NewNamespace(metricAgentScrapeSpansNs).K8sObject(),
			kitk8s.NewNamespace(metricAgentSpansNs).K8sObject(),
			kitk8s.NewNamespace(metricGatewaySpansNs).K8sObject(),
			kitk8s.NewNamespace(metricServiceSpansNs).K8sObject(),
			kitk8s.NewNamespace(traceGatewaySpansNs).K8sObject(),
			kitk8s.NewNamespace(traceServiceSpansNs).K8sObject(),
			kitk8s.NewNamespace(traceServiceInternalSpansNs).K8sObject(),
		)

		mockBackend := backend.New(mockNs, backend.SignalTypeTraces, backend.WithPersistentHostSecret(true))
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, mockBackend.K8sObjects()...)

		pipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-pipeline", mockBackend.Name())).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1())
		pipelineName = pipeline.Name()
		objs = append(objs, pipeline.K8sObject())

		regularSpansGen := telemetrygen.New(regularSpansNs, telemetrygen.SignalTypeTraces).K8sObject()
		vmaScrapeSpansGen := telemetrygen.New(vmaScrapeSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "GET"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Ingress"),
			telemetrygen.WithTelemetryAttribute("user_agent", "vm_promscrape"),
		).K8sObject()
		healthzSpansGen := telemetrygen.New(healthzSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.IstioSystemNamespaceName),
			telemetrygen.WithTelemetryAttribute("http.method", "GET"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "istio-ingressgateway"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Egress"),
			telemetrygen.WithTelemetryAttribute("http.url", "https://healthz.some-url/healthz/ready"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", "istio-system"),
		).K8sObject()
		fluentBitSpansGen := telemetrygen.New(fluentBitSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "fluent-bit"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		metricAgentScrapeSpansGen := telemetrygen.New(metricAgentScrapeSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "GET"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Ingress"),
			telemetrygen.WithTelemetryAttribute("user_agent", "kyma-otelcol/0.1.0"),
		).K8sObject()
		metricAgentSpansGen := telemetrygen.New(metricAgentSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-metric-agent"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		metricGatewaySpansGen := telemetrygen.New(metricGatewaySpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-metric-gateway"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		metricServiceSpansGen := telemetrygen.New(metricServiceSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "POST"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Egress"),
			telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-metrics.kyma-system:4317"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		traceGatewaySpansGen := telemetrygen.New(traceGatewaySpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-trace-gateway"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		traceServiceSpansGen := telemetrygen.New(traceServiceSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "POST"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Egress"),
			telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-traces.kyma-system:4317"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		traceServiceInternalSpansGen := telemetrygen.New(traceServiceInternalSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-trace-collector"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()

		objs = append(objs,
			regularSpansGen,
			vmaScrapeSpansGen,
			healthzSpansGen,
			fluentBitSpansGen,
			metricAgentScrapeSpansGen,
			metricAgentSpansGen,
			metricGatewaySpansGen,
			metricServiceSpansGen,
			traceGatewaySpansGen,
			traceServiceSpansGen,
			traceServiceInternalSpansGen,
		)

		return objs
	}

	Context("When noisy spans are generated", Ordered, func() {
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

		It("Should deliver regular telemetrygen traces", func() {
			verifiers.TracesFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, regularSpansNs)
		})

		It("Should filter noisy spans", func() {
			verifiers.TracesFromNamespacesShouldNotBeDelivered(proxyClient, telemetryExportURL, []string{
				vmaScrapeSpansNs,
				healthzSpansNs,
				fluentBitSpansNs,
				metricAgentScrapeSpansNs,
				metricAgentSpansNs,
				metricGatewaySpansNs,
				metricServiceSpansNs,
				traceGatewaySpansNs,
				traceServiceSpansNs,
				traceServiceInternalSpansNs,
			})
		})
	})
})
