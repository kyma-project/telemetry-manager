package traces

import (
	"context"
	"fmt"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"

	"github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/apiserver"
)

func MakeAndSendTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map) {
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

func MakeAndSendTracesWithAttributes(proxyClient *apiserver.ProxyClient, otlpPushURL string, attributes pcommon.Map, resAttributes pcommon.Map) (pcommon.TraceID, []pcommon.SpanID) {
	traceID := NewTraceID()
	var spanIDs = []pcommon.SpanID{NewSpanID()}

	traces := MakeTraces(traceID, spanIDs, attributes, resAttributes)

	gomega.Expect(sendTraces(context.Background(), proxyClient, traces, otlpPushURL)).To(gomega.Succeed())

	return traceID, spanIDs
}

func MakeAndSendVictoriaMetricsTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "GET")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Ingress")
	attrs.PutStr("user_agent", "vm_promscrape")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func MakeAndSendMetricsEndpointTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "GET")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Ingress")
	attrs.PutStr("http.url", "http://some-url/metrics")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func MakeAndSendHealthzEndpointTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "GET")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Ingress")
	attrs.PutStr("http.url", "http://some-url/healthz")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func MakeAndSendTracePushServiceEndpointTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "POST")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Egress")
	attrs.PutStr("http.url", "http://telemetry-otlp-traces.kyma-system:4317")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func MakeAndSendTraceInternalServiceEndpointTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "POST")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Egress")
	attrs.PutStr("http.url", "http://telemetry-trace-collector-internal.kyma-system:55678")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func MakeAndSendFluentBitServiceTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "POST")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Egress")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("service.name", "telemetry-fluent-bit.kyma-system")

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func MakeAndSendMetricGatewayEgressTraces(proxyClient *apiserver.ProxyClient, otlpPushURL string) (pcommon.TraceID, []pcommon.SpanID, pcommon.Map, pcommon.Map) {
	attrs := pcommon.NewMap()
	attrs.PutStr("http.method", "POST")
	attrs.PutStr("component", "proxy")
	attrs.PutStr("OperationName", "Egress")
	attrs.PutStr("http.url", "http://telemetry-otlp-metrics.kyma-system:4317")

	resAttrs := pcommon.NewMap()
	resAttrs.PutStr("k8s.namespace.name", kitkyma.SystemNamespaceName)

	traceIds, spanIds := MakeAndSendTracesWithAttributes(proxyClient, otlpPushURL, attrs, resAttrs)
	return traceIds, spanIds, attrs, resAttrs
}

func sendTraces(ctx context.Context, proxyClient *apiserver.ProxyClient, traces ptrace.Traces, otlpPushURL string) error {
	sender, err := NewHTTPSender(ctx, otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}

	return sender.Export(ctx, traces)
}
