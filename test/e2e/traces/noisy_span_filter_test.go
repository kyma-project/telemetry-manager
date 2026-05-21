package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/trace"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNoisyFilters(t *testing.T) {
	suite.SetupTest(t, suite.LabelTraces)

	const (
		// regular spans should NOT be filtered
		regularSpansNs = "regular-spans"

		// noisy spans should be filtered
		vmaScrapeSpansNs         = "vma-scrape-spans"
		healthzSpansNs           = "healthz-spans"
		fluentBitSpansNs         = "fluent-bit-spans"
		metricAgentScrapeSpansNs = "metric-agent-scrape-spans"
		metricAgentSpansNs       = "metric-agent-spans"
		metricServiceSpansNs     = "metric-service-spans"
		traceServiceSpansNs      = "trace-service-spans"
		otlpGatewaySpansNs       = "otlp-gateway-spans"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.EndpointHTTP())).
		Build()

	regularSpansGen := telemetrygen.NewPod(regularSpansNs, telemetrygen.SignalTypeTraces).K8sObject()
	vmaScrapeSpansGen := telemetrygen.NewPod(vmaScrapeSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("http.method", "GET"),
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "inbound|80|http://some-url"),
		telemetrygen.WithTelemetryAttribute("user_agent", "vm_promscrape"),
	).K8sObject()
	healthzSpansGen := telemetrygen.NewPod(healthzSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.IstioSystemNamespaceName),
		telemetrygen.WithTelemetryAttribute("http.method", "GET"),
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "istio-ingressgateway"),
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "outbound|80||http://some-url"),
		telemetrygen.WithTelemetryAttribute("http.url", "https://healthz.some-url/healthz/ready"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", "istio-system"),
	).K8sObject()
	fluentBitSpansGen := telemetrygen.NewPod(fluentBitSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-fluent-bit"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()
	metricAgentScrapeSpansGen := telemetrygen.NewPod(metricAgentScrapeSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("http.method", "GET"),
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "inbound||"),
		telemetrygen.WithTelemetryAttribute("user_agent", "kyma-otelcol/0.1.0"),
	).K8sObject()
	metricAgentSpansGen := telemetrygen.NewPod(metricAgentSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-metric-agent"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()
	metricServiceSpansGen := telemetrygen.NewPod(metricServiceSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("http.method", "POST"),
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "outbound||"),
		telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-metrics.kyma-system:4317"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()
	traceServiceSpansGen := telemetrygen.NewPod(traceServiceSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("http.method", "POST"),
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "outbound|80||http://some-url"),
		telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-traces.kyma-system:4317"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()
	otlpGatewaySpansGen := telemetrygen.NewPod(otlpGatewaySpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-otlp-gateway"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()

	resources := []client.Object{
		kitk8sobjects.NewNamespace(backendNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	resources = append(resources,
		kitk8sobjects.NewNamespace(regularSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(vmaScrapeSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(healthzSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(fluentBitSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(metricAgentScrapeSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(metricAgentSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(metricServiceSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(traceServiceSpansNs).K8sObject(),
		kitk8sobjects.NewNamespace(otlpGatewaySpansNs).K8sObject(),
	)

	resources = append(resources,
		regularSpansGen,
		vmaScrapeSpansGen,
		healthzSpansGen,
		fluentBitSpansGen,
		metricAgentScrapeSpansGen,
		metricAgentSpansGen,
		metricServiceSpansGen,
		traceServiceSpansGen,
		otlpGatewaySpansGen,
	)

	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DaemonSetReady(t, kitkyma.OTLPGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	assert.TracesFromNamespaceDelivered(t, backend, regularSpansNs)

	// Spans filtered by rules not depending on namespace (user_agent, http.url, upstream_cluster)
	assert.TracesFromNamespacesNotDelivered(t, backend, []string{
		vmaScrapeSpansNs,
		metricAgentScrapeSpansNs,
		metricServiceSpansNs,
		traceServiceSpansNs,
	})

	// Spans filtered by isTelemetryModuleComponentSpan (namespace=kyma-system + canonical_service).
	// k8s_attributes preserves existing resource attributes, so these spans retain the
	// namespace set by telemetrygen (kyma-system), not the pod's actual namespace.
	// We must assert on the actual resource+span attribute combination.
	assert.BackendDataConsistentlyMatches(t, backend, Not(HaveFlatTraces(ContainElement(
		SatisfyAll(
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.SystemNamespaceName)),
			HaveSpanAttributes(HaveKeyWithValue("istio.canonical_service", BeElementOf(
				"telemetry-otlp-gateway",
				"telemetry-fluent-bit",
				"telemetry-metric-agent",
			))),
		),
	))))

	// Span filtered by isAvailabilityServiceProbeSpan (namespace=istio-system + healthz URL)
	assert.BackendDataConsistentlyMatches(t, backend, Not(HaveFlatTraces(ContainElement(
		SatisfyAll(
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", kitkyma.IstioSystemNamespaceName)),
			HaveSpanAttributes(HaveKeyWithValue("istio.canonical_service", "istio-ingressgateway")),
		),
	))))
}
