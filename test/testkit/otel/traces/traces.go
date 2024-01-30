package traces

import (
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

var (
	ErrInvalidURL       = errors.New("the ProxyURLForService is invalid")
	ErrExporterCreation = errors.New("metric exporter cannot be created")
)

type httpAuthProvider interface {
	TLSConfig() *tls.Config
	Token() string
}

func NewSpanID() pcommon.SpanID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed)) //nolint:gosec // random number generator is sufficient.
	sid := pcommon.SpanID{}
	_, _ = randSource.Read(sid[:])
	return sid
}

func NewTraceID() pcommon.TraceID {
	var rngSeed int64
	_ = binary.Read(crand.Reader, binary.LittleEndian, &rngSeed)
	randSource := rand.New(rand.NewSource(rngSeed)) //nolint:gosec // random number generator is sufficient.
	tid := pcommon.TraceID{}
	_, _ = randSource.Read(tid[:])

	return tid
}

func MakeTraces(traceID pcommon.TraceID, spanIDs []pcommon.SpanID, attributes pcommon.Map, resAttributes pcommon.Map) ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	resAttributes.CopyTo(rs.Resource().Attributes())

	spans := rs.ScopeSpans().
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

func NewHTTPSender(ctx context.Context, otlpURL string, authProvider httpAuthProvider) (exporter Exporter, err error) {
	urlSegments, err := url.Parse(otlpURL)
	if err != nil {
		return exporter, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithTLSClientConfig(authProvider.TLSConfig()),
		otlptracehttp.WithEndpoint(urlSegments.Host),
		otlptracehttp.WithURLPath(urlSegments.Path),
	}

	if len(authProvider.Token()) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(map[string]string{"Authorization": authProvider.Token()}))
	}

	client := otlptracehttp.NewClient(opts...)

	e, err := otlptrace.New(ctx, client)
	if err != nil {
		return exporter, fmt.Errorf("%w: %v", ErrExporterCreation, err)
	}

	return NewExporter(e), nil
}
