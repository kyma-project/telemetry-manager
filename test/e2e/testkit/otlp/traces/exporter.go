package traces

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type Exporter struct {
	otlpExporter *otlptrace.Exporter
}

// NewExporter is an adapter over the OTLP otlptrace.Exporter instance.
func NewExporter(e *otlptrace.Exporter) Exporter {

	return Exporter{otlpExporter: e}
}

func (e Exporter) Export(ctx context.Context, traces ptrace.Traces) error {
	return e.otlpExporter.ExportSpans(ctx, toTraceSpans(traces))
}

func toTraceSpans(traces ptrace.Traces) []tracesdk.ReadOnlySpan {
	var spans []tracesdk.ReadOnlySpan

	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		r := traces.ResourceSpans().At(i)
		for j := 0; j < r.ScopeSpans().Len(); j++ {
			ss := r.ScopeSpans().At(i)
			for k := 0; k < ss.Spans().Len(); k++ {
				s := ss.Spans().At(k)
				spans = append(spans, toSpan(s.TraceID(), s.SpanID(), s.Attributes(), s.StartTimestamp().AsTime()))
			}
		}
	}

	return spans
}

func toSpan(traceID pcommon.TraceID, spanID pcommon.SpanID, attrs pcommon.Map, startTimestamp time.Time) tracesdk.ReadOnlySpan {
	var attributes []attribute.KeyValue
	for k, v := range attrs.AsRaw() {
		attributes = append(attributes, attribute.String(k, v.(string)))
	}

	return tracetest.SpanStub{
		SpanContext: oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
			TraceID: [16]byte(traceID),
			SpanID:  [8]byte(spanID),
		}),
		StartTime:  startTimestamp,
		Attributes: attributes,
	}.Snapshot()
}
