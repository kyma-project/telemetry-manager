//go:build e2e

package traces

import (
	crand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func NewSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))
	sid := pcommon.SpanID{}
	_, _ = randSource.Read(sid[:])
	return sid
}

func NewTraceID() pcommon.TraceID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed))
	tid := pcommon.TraceID{}
	_, _ = randSource.Read(tid[:])
	return tid
}

func MakeTraces(traceID pcommon.TraceID, spanIDs []pcommon.SpanID, attributes pcommon.Map) ptrace.Traces {
	traces := ptrace.NewTraces()

	spans := traces.ResourceSpans().
		AppendEmpty().
		ScopeSpans().
		AppendEmpty().
		Spans()

	for _, spanID := range spanIDs {
		span := spans.AppendEmpty()
		span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		span.SetSpanID(spanID)
		span.SetTraceID(traceID)
		attributes.CopyTo(span.Attributes())
	}

	return traces
}
