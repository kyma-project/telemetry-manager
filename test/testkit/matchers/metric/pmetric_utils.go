package metric

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

func extractMetrics(fileBytes []byte) ([]pmetric.Metrics, error) {
	mds, err := unmarshalMetrics(fileBytes)
	if err != nil {
		return nil, err
	}

	applyTemporalityWorkaround(mds)

	return mds, nil
}

func unmarshalMetrics(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
	return matchers.UnmarshalSignals[pmetric.Metrics](jsonlMetrics, func(buf []byte) (pmetric.Metrics, error) {
		var unmarshaler pmetric.JSONUnmarshaler
		return unmarshaler.UnmarshalMetrics(buf)
	})
}

// applyTemporalityWorkaround flips temporality os a Sum metric. The reason for that is the inconsistency
// between the metricdata package (https://github.com/open-telemetry/opentelemetry-go/blob/main/sdk/metric/metricdata/temporality.go)
// and the pmetric package (https://github.com/open-telemetry/opentelemetry-collector/blob/main/pdata/pmetric/aggregation_temporality.go)
func applyTemporalityWorkaround(mds []pmetric.Metrics) {
	for _, md := range mds {
		for _, metric := range getMetrics(md) {
			if metric.Type() != pmetric.MetricTypeSum {
				continue
			}

			if metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			} else {
				metric.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
		}
	}
}

func getMetrics(md pmetric.Metrics) []pmetric.Metric {
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
