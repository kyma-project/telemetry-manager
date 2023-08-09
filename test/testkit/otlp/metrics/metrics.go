//go:build e2e

package metrics

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math/rand"
	neturl "net/url"
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
	setGaugeDefaults(m)
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

func NewCumulativeSum(opts ...MetricOption) pmetric.Metric {
	startTime := time.Now()
	totalPts := 2
	totalAttributes := 7

	m := pmetric.NewMetric()
	setCumulativeSumDefaults(m)
	for _, opt := range opts {
		opt(m)
	}

	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	pts := sum.DataPoints()
	for i := 0; i < totalPts; i++ {
		pt := pts.AppendEmpty()
		pt.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		pt.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
		pt.SetDoubleValue(float64(i))

		for i := 0; i < totalAttributes; i++ {
			k := fmt.Sprintf("pt-label-key-%d", i)
			v := fmt.Sprintf("pt-label-val-%d", i)
			pt.Attributes().PutStr(k, v)
		}
	}

	return m
}

func setCumulativeSumDefaults(m pmetric.Metric) {
	m.SetName("dummy_cumulative_sum")
	m.SetDescription("Dummy cumulative sum")
	m.SetUnit("ms")
}

func setGaugeDefaults(m pmetric.Metric) {
	m.SetName("dummy_gauge")
	m.SetDescription("Dummy gauge")
	m.SetUnit("ms")
}

func AllMetrics(md pmetric.Metrics) []pmetric.Metric {
	var metrics []pmetric.Metric

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metrics = append(metrics, scopeMetrics.Metrics().At(k))
			}
		}
	}

	return metrics
}

func AllMetricNames(md pmetric.Metrics) []string {
	var metricNames []string

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metricNames = append(metricNames, scopeMetrics.Metrics().At(k).Name())
			}
		}
	}

	return makeUnique(metricNames)
}

func AllResourceAttributeNames(md pmetric.Metrics) []string {
	var attributes []string

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for key := range resourceMetrics.Resource().Attributes().AsRaw() {
			attributes = append(attributes, key)
		}
	}
	return makeUnique(attributes)
}

func makeUnique(slice []string) []string {
	keys := make(map[string]bool)
	uniqueList := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			uniqueList = append(uniqueList, entry)
		}
	}
	return uniqueList
}

func AllDataPointsContainAttributes(m pmetric.Metric, expectedAttrKeys ...string) bool {
	attrsPerDataPoint := getAttributesPerDataPoint(m)
	for _, attrs := range attrsPerDataPoint {
		if !containsAllAttributes(attrs, expectedAttrKeys...) {
			return false
		}
	}

	return true
}

func NoDataPointsContainAttributes(m pmetric.Metric, expectedAttrKeys ...string) bool {
	attrsPerDataPoint := getAttributesPerDataPoint(m)
	for _, attrs := range attrsPerDataPoint {
		if !containsNoneOfAttributes(attrs, expectedAttrKeys...) {
			return false
		}
	}

	return true
}

func getAttributesPerDataPoint(m pmetric.Metric) []pcommon.Map {
	var attrsPerDataPoint []pcommon.Map

	switch m.Type() {
	case pmetric.MetricTypeSum:
		for i := 0; i < m.Sum().DataPoints().Len(); i++ {
			attrsPerDataPoint = append(attrsPerDataPoint, m.Sum().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeGauge:
		for i := 0; i < m.Gauge().DataPoints().Len(); i++ {
			attrsPerDataPoint = append(attrsPerDataPoint, m.Gauge().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < m.Histogram().DataPoints().Len(); i++ {
			attrsPerDataPoint = append(attrsPerDataPoint, m.Histogram().DataPoints().At(i).Attributes())
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < m.Summary().DataPoints().Len(); i++ {
			attrsPerDataPoint = append(attrsPerDataPoint, m.Summary().DataPoints().At(i).Attributes())
		}
	}

	return attrsPerDataPoint
}

func containsAllAttributes(m pcommon.Map, attrKeys ...string) bool {
	for _, key := range attrKeys {
		if _, found := m.Get(key); !found {
			return false
		}
	}
	return true
}

func containsNoneOfAttributes(m pcommon.Map, attrKeys ...string) bool {
	for _, key := range attrKeys {
		if _, found := m.Get(key); found {
			return false
		}
	}
	return true
}

func NewHTTPExporter(url string, authProvider httpAuthProvider) (exporter Exporter, err error) {
	urlSegments, err := neturl.Parse(url)
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
