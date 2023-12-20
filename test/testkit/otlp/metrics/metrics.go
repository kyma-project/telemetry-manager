package metrics

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
)

var (
	ErrInvalidURL       = errors.New("the ProxyURLForService is invalid")
	ErrExporterCreation = errors.New("metric exporter cannot be created")
)

type httpAuthProvider interface {
	TLSConfig() *tls.Config
	Token() string
}

type Builder struct {
	metrics []pmetric.Metric
}

func NewBuilder() *Builder {
	return &Builder{}
}

func (b *Builder) WithMetric(m pmetric.Metric) *Builder {
	b.metrics = append(b.metrics, m)
	return b
}

func (b *Builder) Build() pmetric.Metrics {
	md := pmetric.NewMetrics()
	scopeMetric := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for _, metrics := range b.metrics {
		metrics.CopyTo(scopeMetric.Metrics().AppendEmpty())
	}
	return md
}

type MetricOption = func(pmetric.Metric)

func WithName(name string) MetricOption {
	return func(m pmetric.Metric) {
		m.SetName(name)
	}
}

func NewGauge(opts ...MetricOption) pmetric.Metric {
	totalAttributes := 7
	totalPts := 2
	startTime := time.Now()

	m := pmetric.NewMetric()
	setMetricDefaults(m)
	for _, opt := range opts {
		opt(m)
	}

	gauge := m.SetEmptyGauge()
	pts := gauge.DataPoints()
	for i := 0; i < totalPts; i++ {
		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetDoubleValue(rand.Float64()) //nolint:gosec // random number generator is sufficient.

		for i := 0; i < totalAttributes; i++ {
			k := fmt.Sprintf("pt-label-key-%d", i)
			v := fmt.Sprintf("pt-label-val-%d", i)
			pt.Attributes().PutStr(k, v)
		}
	}

	return m
}

func setMetricDefaults(m pmetric.Metric) {
	m.SetName("dummy_gauge")
	m.SetDescription("Dummy gauge")
	m.SetUnit("ms")
}

func NewHTTPExporter(otlpURL string, authProvider httpAuthProvider) (exporter Exporter, err error) {
	urlSegments, err := url.Parse(otlpURL)
	if err != nil {
		return exporter, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithTLSClientConfig(authProvider.TLSConfig()),
		otlpmetrichttp.WithEndpoint(urlSegments.Host),
		otlpmetrichttp.WithURLPath(urlSegments.Path),
	}

	if len(authProvider.Token()) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(map[string]string{"Authorization": authProvider.Token()}))
	}

	e, err := otlpmetrichttp.New(context.TODO(), opts...)
	if err != nil {
		return exporter, fmt.Errorf("%w: %v", ErrExporterCreation, err)
	}

	return NewExporter(e), nil
}
