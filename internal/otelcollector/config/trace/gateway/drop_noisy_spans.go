package gateway

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

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

	operationIsIngress = config.JoinWithOr(spanAttributeEquals("OperationName", "Ingress"), attributeMatches("name", "ingress.*"))
	operationIsEgress  = config.JoinWithOr(spanAttributeEquals("OperationName", "Egress"), attributeMatches("name", "egress.*"))

	toFromKymaGrafana            = config.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("grafana"))
	toFromKymaAuthProxy          = config.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("monitoring-auth-proxy-grafana"))
	toFromTelemetryFluentBit     = config.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-fluent-bit"))
	toFromTelemetryTraceGateway  = config.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-trace-collector"))
	toFromTelemetryMetricGateway = config.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-gateway"))
	toFromTelemetryMetricAgent   = config.JoinWithAnd(componentIsProxy, namespacesIsKymaSystem, istioCanonicalNameEquals("telemetry-metric-agent"))

	toIstioGatewayWithHealthz = config.JoinWithAnd(componentIsProxy, namespacesIsIstioSystem, methodIsGet, operationIsEgress, istioCanonicalNameEquals("istio-ingressgateway"), urlIsIstioHealthz)

	toTelemetryTraceService         = config.JoinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryTraceService)
	toTelemetryTraceInternalService = config.JoinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryTraceInternalService)
	toTelemetryMetricService        = config.JoinWithAnd(componentIsProxy, methodIsPost, operationIsEgress, urlIsTelemetryMetricService)

	//TODO: should be system namespaces after solving https://github.com/kyma-project/telemetry-manager/issues/380
	fromVMScrapeAgent        = config.JoinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, userAgentMatches("vm_promscrape"))
	fromPrometheusWithinKyma = config.JoinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, namespacesIsKymaSystem, userAgentMatches("Prometheus\\\\/.*"))
	fromTelemetryMetricAgent = config.JoinWithAnd(componentIsProxy, methodIsGet, operationIsIngress, userAgentMatches("kyma-otelcol\\\\/.*"))
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

func namespaceEquals(name string) string {
	return config.ResourceAttributeEquals("k8s.namespace.name", name)
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
