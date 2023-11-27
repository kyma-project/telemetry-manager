package gateway

import (
	. "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

var (
	namespacesIsKymaSystem  = NamespaceEquals("kyma-system")
	namespacesIsIstioSystem = NamespaceEquals("istio-system")
	methodIsGet             = spanAttributeEquals("http.method", "GET")
	methodIsPost            = spanAttributeEquals("http.method", "POST")
	componentIsProxy        = spanAttributeEquals("component", "proxy")

	urlIsIstioHealthz                  = urlMatches("https:\\\\/\\\\/healthz\\\\..+\\\\/healthz\\\\/ready")
	urlIsTelemetryTraceService         = urlMatches("http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(4317|4318).*")
	urlIsTelemetryTraceInternalService = urlMatches("http(s)?:\\\\/\\\\/telemetry-trace-collector-internal\\\\.kyma-system(\\\\..*)?:(55678).*")
	urlIsTelemetryMetricService        = urlMatches("http(s)?:\\\\/\\\\/telemetry-otlp-metrics\\\\.kyma-system(\\\\..*)?:(4317|4318).*")

	operationIsIngress = JoinWithOr(spanAttributeEquals("OperationName", "Ingress"), attributeMatches("name", "ingress.*"))
	operationIsEgress  = JoinWithOr(spanAttributeEquals("OperationName", "Egress"), attributeMatches("name", "egress.*"))

	toFromKymaGrafana            = JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("grafana"))
	toFromKymaAuthProxy          = JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("monitoring-auth-proxy-grafana"))
	toFromTelemetryFluentBit     = JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-fluent-bit"))
	toFromTelemetryTraceGateway  = JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-trace-collector"))
	toFromTelemetryMetricGateway = JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-gateway"))
	toFromTelemetryMetricAgent   = JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-agent"))

	toIstioGatewayWithHealthz = JoinWithAnd(componentIsProxy, namespacesIsIstioSystem, methodIsGet, operationIsEgress, istioCanonicalNameEquals("istio-ingressgateway"), urlIsIstioHealthz)

	toTelemetryTraceService         = JoinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryTraceService)
	toTelemetryTraceInternalService = JoinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryTraceInternalService)
	toTelemetryMetricService        = JoinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryMetricService)

	//TODO: should be system namespaces after solving https://github.com/kyma-project/telemetry-manager/issues/380
	fromVMScrapeAgent        = JoinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, userAgentMatches("vm_promscrape"))
	fromPrometheusWithinKyma = JoinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, namespacesIsKymaSystem, userAgentMatches("Prometheus\\\\/.*"))
	fromTelemetryMetricAgent = JoinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, userAgentMatches("kyma-otelcol\\\\/.*"))
)

func makeDropNoisySpansConfig() FilterProcessor {
	return FilterProcessor{
		Traces: Traces{
			Span: []string{
				toFromKymaGrafana,
				toFromKymaAuthProxy,
				toFromTelemetryFluentBit,
				toFromTelemetryTraceGateway,
				toFromTelemetryMetricGateway,
				toFromTelemetryMetricAgent,
				toIstioGatewayWithHealthz,
				toTelemetryTraceService,
				toTelemetryTraceInternalService,
				toTelemetryMetricService,
				fromVMScrapeAgent,
				fromPrometheusWithinKyma,
				fromTelemetryMetricAgent,
			},
		},
	}
}

func spanAttributeEquals(key, value string) string {
	return "attributes[\"" + key + "\"] == \"" + value + "\""
}

func istioCanonicalNameEquals(name string) string {
	return spanAttributeEquals("istio.canonical_service", name)
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
