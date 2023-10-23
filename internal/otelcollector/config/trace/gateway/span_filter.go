package gateway

import (
	"strings"
)

var (
	namespacesIsKymaSystem  = namespaceEquals("kyma-system")
	namespacesIsIstioSystem = namespaceEquals("istio-system")
	methodIsGet             = spanAttributeEquals("http.method", "GET")
	methodIsPost            = spanAttributeEquals("http.method", "POST")
	componentIsProxy        = spanAttributeEquals("component", "proxy")

	urlIsIstioHealthz                  = urlMatches("https:\\\\/\\\\/healthz\\\\..+\\\\/healthz\\\\/ready")
	urlIsTelemetryTraceService         = urlMatches("http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(4317|4318).*")
	urlIsTelemetryTraceInternalService = urlMatches("http(s)?:\\\\/\\\\/telemetry-trace-collector-internal\\\\.kyma-system(\\\\..*)?:(55678).*")
	urlIsTelemetryMetricService        = urlMatches("http(s)?:\\\\/\\\\/telemetry-otlp-metrics\\\\.kyma-system(\\\\..*)?:(4317|4318).*")

	operationIsIngress = joinWithOr(spanAttributeEquals("OperationName", "Ingress"), attributeMatches("name", "ingress.*"))
	operationIsEgress  = joinWithOr(spanAttributeEquals("OperationName", "Egress"), attributeMatches("name", "egress.*"))

	toFromKymaGrafana            = joinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("grafana"))
	toFromKymaAuthProxy          = joinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("monitoring-auth-proxy-grafana"))
	toFromTelemetryFluentBit     = joinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-fluent-bit"))
	toFromTelemetryTraceGateway  = joinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-trace-collector"))
	toFromTelemetryMetricGateway = joinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-gateway"))
	toFromTelemetryMetricAgent   = joinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-agent"))

	toIstioGatewayWitHealthz = joinWithAnd(componentIsProxy, namespacesIsIstioSystem, methodIsGet, operationIsEgress, istioCanonicalNameEquals("istio-ingressgateway"), urlIsIstioHealthz)

	toTelemetryTraceService         = joinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryTraceService)
	toTelemetryTraceInternalService = joinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryTraceInternalService)
	toTelemetryMetricService        = joinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryMetricService)

	//TODO: should be system namespaces after solving https://github.com/kyma-project/telemetry-manager/issues/380
	fromVMScrapeAgent        = joinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, userAgentMatches("vm_promscrape"))
	fromPrometheusWithinKyma = joinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, namespacesIsKymaSystem, userAgentMatches("Prometheus\\\\/.*"))
	fromTelemetryMetricAgent = joinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, userAgentMatches("kyma-otelcol\\\\/.*"))
)

func makeSpanFilterConfig() []string {
	return []string{
		toFromKymaGrafana,
		toFromKymaAuthProxy,
		toFromTelemetryFluentBit,
		toFromTelemetryTraceGateway,
		toFromTelemetryMetricGateway,
		toFromTelemetryMetricAgent,
		toIstioGatewayWitHealthz,
		toTelemetryTraceService,
		toTelemetryTraceInternalService,
		toTelemetryMetricService,
		fromVMScrapeAgent,
		fromPrometheusWithinKyma,
		fromTelemetryMetricAgent,
	}
}

func spanAttributeEquals(key, value string) string {
	return "attributes[\"" + key + "\"] == \"" + value + "\""
}

func istioCanonicalNameEquals(name string) string {
	return spanAttributeEquals("istio.canonical_service", name)
}

func namespaceEquals(name string) string {
	return resourceAttributeEquals("k8s.namespace.name", name)
}
func resourceAttributeEquals(key, value string) string {
	return "resource.attributes[\"" + key + "\"] == \"" + value + "\""
}

func urlMatches(pattern string) string {
	return spanAttributeMatches("http.url", pattern)
}

func userAgentMatches(pattern string) string {
	return spanAttributeMatches("user_agent", pattern)
}

func spanAttributeMatches(key, pattern string) string {
	return attributeMatches("attributes[\""+key+"\"]", pattern)
}

func attributeMatches(key, pattern string) string {
	return "IsMatch(" + key + ", \"" + pattern + "\") == true"
}

func joinWithAnd(parts ...string) string {
	return strings.Join(parts, " and ")
}

func joinWithOr(parts ...string) string {
	return "(" + strings.Join(parts, " or ") + ")"
}
