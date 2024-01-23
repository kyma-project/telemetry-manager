package traces

import (
	"context"
	"fmt"

	"github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
)

func MakeAndSendTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map) {
	traceID := NewTraceID()
	var spanIDs []pcommon.SpanID
	for i := 0; i < 100; i++ {
		spanIDs = append(spanIDs, NewSpanID())
	}

	attrs := pcommon.NewMap()
	attrs.PutStr("attrA", "chocolate")
	attrs.PutStr("attrB", "raspberry")
	attrs.PutStr("attrC", "vanilla")
	traces := MakeTraces(traceID, spanIDs, attrs, pcommon.NewMap())

	gomega.Expect(sendTraces(context.Background(), proxyClient, traces, otlpPushURL)).To(gomega.Succeed())

	return traceID, spanIDs, attrs
}

func MakeAndSendTracesWithAttributes(proxyClient *apiserverproxy.Client, otlpPushURL string, attributes pcommon.Map, resAttributes pcommon.Map) (pcommon.TraceID, []pcommon.SpanID) {
	traceID := NewTraceID()
	var spanIDs = []pcommon.SpanID{NewSpanID()}

	traces := MakeTraces(traceID, spanIDs, attributes, resAttributes)

	gomega.Expect(sendTraces(context.Background(), proxyClient, traces, otlpPushURL)).To(gomega.Succeed())

	return traceID, spanIDs
}

func MakeAndSendVictoriaMetricsAgentTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("http.method", "GET")
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("OperationName", "Ingress")
	spanAttrs.PutStr("user_agent", "vm_promscrape")

	resourceAttrs := pcommon.NewMap()

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendMetricAgentScrapeTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("http.method", "GET")
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("OperationName", "Ingress")
	spanAttrs.PutStr("user_agent", "kyma-otelcol/0.1.0")

	resourceAttrs := pcommon.NewMap()

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendIstioHealthzEndpointTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("http.method", "GET")
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("istio.canonical_service", "istio-ingressgateway")
	spanAttrs.PutStr("OperationName", "Egress")
	spanAttrs.PutStr("http.url", "https://healthz.some-url/healthz/ready")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.IstioSystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendTraceServiceTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("http.method", "POST")
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("OperationName", "Egress")
	spanAttrs.PutStr("http.url", "http://telemetry-otlp-traces.kyma-system:4317")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendTraceInternalServiceTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("http.method", "POST")
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("OperationName", "Egress")
	spanAttrs.PutStr("http.url", "http://telemetry-trace-collector-internal.kyma-system:55678")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendMetricServiceTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("http.method", "POST")
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("OperationName", "Egress")
	spanAttrs.PutStr("http.url", "http://telemetry-otlp-metrics.kyma-system:4317")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendTraceGatewayTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("istio.canonical_service", "telemetry-trace-collector")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendMetricGatewayTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("istio.canonical_service", "telemetry-metric-gateway")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendMetricAgentTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("istio.canonical_service", "telemetry-metric-agent")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func MakeAndSendFluentBitTraces(proxyClient *apiserverproxy.Client, otlpPushURL string) pcommon.TraceID {
	spanAttrs := pcommon.NewMap()
	spanAttrs.PutStr("component", "proxy")
	spanAttrs.PutStr("istio.canonical_service", "telemetry-fluent-bit")

	resourceAttrs := pcommon.NewMap()
	resourceAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceID, _ := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, spanAttrs, resourceAttrs)
	return traceID
}

func sendTraces(ctx context.Context, proxyClient *apiserverproxy.Client, traces ptrace.Traces, otlpPushURL string) error {
	sender, err := NewHTTPSender(ctx, otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}

	return sender.Export(ctx, traces)
}
