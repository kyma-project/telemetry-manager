package trace

import (
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto

func generateTd() ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	attrs := rs.Resource().Attributes()
	attrs.PutStr("k8s.cluster.name", "cluster-01")
	attrs.PutStr("k8s.deployment.name", "nginx")

	spans := rs.ScopeSpans().AppendEmpty().Spans()
	traceID := traces.NewTraceID()

	span1 := spans.AppendEmpty()
	span1.SetTraceID(traceID)
	span1.SetSpanID(traces.NewSpanID())
	attrs1 := span1.Attributes()
	attrs1.PutStr("color", "red")

	span2 := spans.AppendEmpty()
	span2.SetTraceID(traceID)
	span2.SetSpanID(traces.NewSpanID())
	attrs2 := span2.Attributes()
	attrs2.PutStr("color", "blue")

	return td
}

var jsonlTraces = mustMarshalTraces(generateTd())

var _ = Describe("Demo-1", func() {
	It("tds not empty", func() {
		Expect(jsonlTraces).Should(WithTransform(func(jsonlTraces []byte) ([]ptrace.Traces, error) {
			tds, err := unmarshalTraces(jsonlTraces)
			if err != nil {
				return nil, err
			}

			return tds, nil
		}, ContainElements()))
	})
})
