package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

var (
	namespacesIsKymaSystem  = ottlexpr.NamespaceEquals("kyma-system")
	namespacesIsIstioSystem = ottlexpr.NamespaceEquals("istio-system")
	methodIsGet             = spanAttributeEquals("http.method", "GET")
	methodIsPost            = spanAttributeEquals("http.method", "POST")
	componentIsProxy        = spanAttributeEquals("component", "proxy")

	urlIsIstioHealthz           = urlMatches("https:\\\\/\\\\/healthz\\\\..+\\\\/healthz\\\\/ready")
	urlIsTelemetryTraceService  = urlMatches("http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(4317|4318).*")
	urlIsTelemetryMetricService = urlMatches("http(s)?:\\\\/\\\\/telemetry-otlp-metrics\\\\.kyma-system(\\\\..*)?:(4317|4318).*")

	operationIsInbound  = spanAttributeMatches("upstream_cluster.name", "inbound|.+")
	operationIsOutbound = spanAttributeMatches("upstream_cluster.name", "outbound|.+")

	toFromTelemetryFluentBit     = ottlexpr.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-fluent-bit"))
	toFromTelemetryTraceGateway  = ottlexpr.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-trace-gateway"))
	toFromTelemetryMetricGateway = ottlexpr.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-gateway"))
	toFromTelemetryMetricAgent   = ottlexpr.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-agent"))

	toIstioGatewayWithHealthz = ottlexpr.JoinWithAnd(componentIsProxy, namespacesIsIstioSystem, methodIsGet, operationIsOutbound, istioCanonicalNameEquals("istio-ingressgateway"), urlIsIstioHealthz)

	toTelemetryTraceService  = ottlexpr.JoinWithAnd(componentIsProxy, methodIsPost, operationIsOutbound, urlIsTelemetryTraceService)
	toTelemetryMetricService = ottlexpr.JoinWithAnd(componentIsProxy, methodIsPost, operationIsOutbound, urlIsTelemetryMetricService)

	//TODO: should be system namespaces after solving https://github.com/kyma-project/telemetry-manager/issues/380
	fromVMScrapeAgent        = ottlexpr.JoinWithAnd(componentIsProxy, methodIsGet, operationIsInbound, userAgentMatches("vm_promscrape"))
	fromTelemetryMetricAgent = ottlexpr.JoinWithAnd(componentIsProxy, methodIsGet, operationIsInbound, userAgentMatches("kyma-otelcol\\\\/.*"))
)

func makeDropNoisySpansConfig() FilterProcessor {
	return FilterProcessor{
		Traces: Traces{
			Span: []string{
				toFromTelemetryFluentBit,
				toFromTelemetryTraceGateway,
				toFromTelemetryMetricGateway,
				toFromTelemetryMetricAgent,
				toIstioGatewayWithHealthz,
				toTelemetryTraceService,
				toTelemetryMetricService,
				fromVMScrapeAgent,
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
