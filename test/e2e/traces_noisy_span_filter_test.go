//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	const (
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

		backend := backend.New(mockNs, backend.SignalTypeTraces, backend.WithPersistentHostSecret(true))
		backendExportURL = backend.ExportURL(proxyClient)
		objs = append(objs, backend.K8sObjects()...)

		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &tracePipeline)

		regularSpansGen := telemetrygen.NewPod(regularSpansNs, telemetrygen.SignalTypeTraces).K8sObject()
		vmaScrapeSpansGen := telemetrygen.NewPod(vmaScrapeSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "GET"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Ingress"),
			telemetrygen.WithTelemetryAttribute("user_agent", "vm_promscrape"),
		).K8sObject()
		healthzSpansGen := telemetrygen.NewPod(healthzSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.IstioSystemNamespaceName),
			telemetrygen.WithTelemetryAttribute("http.method", "GET"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "istio-ingressgateway"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Egress"),
			telemetrygen.WithTelemetryAttribute("http.url", "https://healthz.some-url/healthz/ready"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", "istio-system"),
		).K8sObject()
		fluentBitSpansGen := telemetrygen.NewPod(fluentBitSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "fluent-bit"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		metricAgentScrapeSpansGen := telemetrygen.NewPod(metricAgentScrapeSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "GET"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Ingress"),
			telemetrygen.WithTelemetryAttribute("user_agent", "kyma-otelcol/0.1.0"),
		).K8sObject()
		metricAgentSpansGen := telemetrygen.NewPod(metricAgentSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-metric-agent"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		metricGatewaySpansGen := telemetrygen.NewPod(metricGatewaySpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-metric-gateway"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		metricServiceSpansGen := telemetrygen.NewPod(metricServiceSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "POST"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Egress"),
			telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-metrics.kyma-system:4317"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		traceGatewaySpansGen := telemetrygen.NewPod(traceGatewaySpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-trace-gateway"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		traceServiceSpansGen := telemetrygen.NewPod(traceServiceSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("http.method", "POST"),
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("OperationName", "Egress"),
			telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-traces.kyma-system:4317"),
			telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
		).K8sObject()
		traceServiceInternalSpansGen := telemetrygen.NewPod(traceServiceInternalSpansNs, telemetrygen.SignalTypeTraces,
			telemetrygen.WithTelemetryAttribute("component", "proxy"),
			telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-trace-gateway"),
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
			assert.TracePipelineHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should have a running trace gateway deployment", func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have a trace backend running", func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should deliver regular telemetrygen traces", func() {
			assert.TracesFromNamespaceDelivered(proxyClient, backendExportURL, regularSpansNs)
		})

		It("Should filter noisy spans", func() {
			assert.TracesFromNamespacesNotDelivered(proxyClient, backendExportURL, []string{
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
