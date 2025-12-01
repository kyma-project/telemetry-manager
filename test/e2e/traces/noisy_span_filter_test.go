package traces

import (
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestNoisyFilters(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

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

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		backendNs    = uniquePrefix("backend")
	)

	backend := kitbackend.New(backendNs, kitbackend.SignalTypeTraces)

	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
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
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "fluent-bit"),
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
	metricGatewaySpansGen := telemetrygen.NewPod(metricGatewaySpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-metric-gateway"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()
	metricServiceSpansGen := telemetrygen.NewPod(metricServiceSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("http.method", "POST"),
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "outbound||"),
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
		telemetrygen.WithTelemetryAttribute("upstream_cluster.name", "outbound|80||http://some-url"),
		telemetrygen.WithTelemetryAttribute("http.url", "http://telemetry-otlp-traces.kyma-system:4317"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()
	traceServiceInternalSpansGen := telemetrygen.NewPod(traceServiceInternalSpansNs, telemetrygen.SignalTypeTraces,
		telemetrygen.WithTelemetryAttribute("component", "proxy"),
		telemetrygen.WithTelemetryAttribute("istio.canonical_service", "telemetry-trace-gateway"),
		telemetrygen.WithResourceAttribute("k8s.namespace.name", kitkyma.SystemNamespaceName),
	).K8sObject()

	resources := []client.Object{
		objects.NewNamespace(backendNs).K8sObject(),
		&pipeline,
	}
	resources = append(resources, backend.K8sObjects()...)

	resources = append(resources,
		objects.NewNamespace(regularSpansNs).K8sObject(),
		objects.NewNamespace(vmaScrapeSpansNs).K8sObject(),
		objects.NewNamespace(healthzSpansNs).K8sObject(),
		objects.NewNamespace(fluentBitSpansNs).K8sObject(),
		objects.NewNamespace(metricAgentScrapeSpansNs).K8sObject(),
		objects.NewNamespace(metricAgentSpansNs).K8sObject(),
		objects.NewNamespace(metricGatewaySpansNs).K8sObject(),
		objects.NewNamespace(metricServiceSpansNs).K8sObject(),
		objects.NewNamespace(traceGatewaySpansNs).K8sObject(),
		objects.NewNamespace(traceServiceSpansNs).K8sObject(),
		objects.NewNamespace(traceServiceInternalSpansNs).K8sObject(),
	)

	resources = append(resources,
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

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

	assert.BackendReachable(t, backend)
	assert.DeploymentReady(t, kitkyma.TraceGatewayName)
	assert.TracePipelineHealthy(t, pipelineName)

	assert.TracesFromNamespaceDelivered(t, backend, regularSpansNs)

	assert.TracesFromNamespacesNotDelivered(t, backend, []string{
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
}
