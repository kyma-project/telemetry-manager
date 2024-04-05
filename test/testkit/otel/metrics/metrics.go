package metrics

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var (
	ErrInvalidURL       = errors.New("the ProxyURLForService is invalid")
	ErrExporterCreation = errors.New("metric exporter cannot be created")
)

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
