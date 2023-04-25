//go:build e2e

package metrics

import (
	"fmt"
	"math/rand"
	"net/url"
	"strconv"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Builder struct {
	metrics []pmetric.Metric
}

func NewBuilder() *Builder {
	return &Builder{}
}

type MetricOption = func(pmetric.Metric)

func WithName(name string) MetricOption {
	return func(m pmetric.Metric) {
		m.SetName(name)
	}
}

func (b *Builder) WithGauge(gauge pmetric.Gauge, opts ...MetricOption) *Builder {
	m := pmetric.NewMetric()
	setMetricDefaults(m)

	gauge.CopyTo(m.SetEmptyGauge())
	for _, opt := range opts {
		opt(m)
	}

	b.metrics = append(b.metrics, m)
	return b
}

func setMetricDefaults(m pmetric.Metric) {
	m.SetName("dummy_gauge")
	m.SetDescription("Dummy gauge")
	m.SetUnit("ms")
}

func (b *Builder) Build() pmetric.Metrics {
	md := pmetric.NewMetrics()
	scopeMetric := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	for _, metrics := range b.metrics {
		metrics.CopyTo(scopeMetric.Metrics().AppendEmpty())
	}
	return md
}

func NewGauge() pmetric.Gauge {
	totalAttributes := 7
	totalPts := 2
	startTime := time.Now()

	gauge := pmetric.NewGauge()
	pts := gauge.DataPoints()
	for i := 0; i < totalPts; i++ {
		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetDoubleValue(rand.Float64())

		for i := 0; i < totalAttributes; i++ {
			k := fmt.Sprintf("pt-label-key-%d", i)
			v := fmt.Sprintf("pt-label-val-%d", i)
			pt.Attributes().PutStr(k, v)
		}
	}

	return gauge
}

func AllGauges(md pmetric.Metrics) []pmetric.Gauge {
	var gauges []pmetric.Gauge

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				gauges = append(gauges, scopeMetrics.Metrics().At(k).Gauge())
			}
		}
	}

	return gauges
}

func NewDataSender(otlpPushURL string) (testbed.MetricDataSender, error) {
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
		return testbed.NewOTLPMetricDataSender(host, port), nil
	}

	if typedURL.Scheme == "https" {
		return testbed.NewOTLPHTTPMetricDataSender(host, port), nil
	}

	return nil, fmt.Errorf("unsupported url scheme: %s", typedURL.Scheme)
}
