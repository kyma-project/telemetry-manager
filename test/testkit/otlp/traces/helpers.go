package traces

import (
	"context"
	"fmt"

	. "github.com/onsi/gomega"
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
	traces := MakeTraces(traceID, spanIDs, attrs)

	Expect(sendTraces(context.Background(), proxyClient, traces, otlpPushURL)).To(Succeed())

	return traceID, spanIDs, attrs
}

func sendTraces(ctx context.Context, proxyClient *apiserver.ProxyClient, traces ptrace.Traces, otlpPushURL string) error {
	sender, err := NewHTTPSender(ctx, otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}

	return sender.Export(ctx, traces)
}
