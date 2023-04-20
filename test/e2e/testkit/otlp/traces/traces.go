//go:build e2e

package traces

import (
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
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

func MakeDataSender(otlpPushURL string) (testbed.TraceDataSender, error) {
	typedURL, err := url.Parse(otlpPushURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse url: %v", err)
	}

	host := typedURL.Hostname()
	port, err := strconv.Atoi(typedURL.Port())
	if err != nil {
		return nil, fmt.Errorf("failed to parse port: %v", err)
	}

	if typedURL.Scheme == "grpc" {
		return testbed.NewOTLPTraceDataSender(host, port), nil
	}

	if typedURL.Scheme == "https" {
		return testbed.NewOTLPHTTPTraceDataSender(host, port), nil
	}

	return nil, fmt.Errorf("unsupported url scheme: %s", typedURL.Scheme)
}
