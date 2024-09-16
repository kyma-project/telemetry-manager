package metric

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

// FlatMetric holds all needed information about a metric data point.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with the deeply nested structure of pmetrics.
//
// Introducing a go struct with a flat data structure by extracting necessary information from different levels of pmetrics makes accessing the information easier than using pmetric.Metrics directly and improves the readability of the test output logs.
type FlatMetric struct {
	Name, Description, ScopeName, ScopeVersion            string
	ResourceAttributes, ScopeAttributes, MetricAttributes map[string]string
	Type                                                  string
}

func unmarshalMetrics(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
	return matchers.UnmarshalSignals[pmetric.Metrics](jsonlMetrics, func(buf []byte) (pmetric.Metrics, error) {
		var unmarshaler pmetric.JSONUnmarshaler
		return unmarshaler.UnmarshalMetrics(buf)
	})
}

// flattenAllMetrics flattens an array of pdata.Metrics datapoints to a slice of FlatMetric.
// It converts the deeply nested pdata.Metrics data structure to a flat struct, to make it more readable in the test output logs.
func flattenAllMetrics(mds []pmetric.Metrics) []FlatMetric {
	var flatMetrics []FlatMetric

	for _, md := range mds {
		flatMetrics = append(flatMetrics, flattenMetrics(md)...)
	}

	return flatMetrics
}

// flattenMetrics converts a single pdata.Metrics datapoint to a slice of FlatMetric
// It takes relevant information from different levels of pdata and puts it into a FlatMetric go struct.
func flattenMetrics(md pmetric.Metrics) []FlatMetric {
	var flatMetrics []FlatMetric

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)
				dataPointsAttributes := getAttributesPerDataPoint(metric)
				for l := 0; l < len(dataPointsAttributes); l++ {
					flatMetrics = append(flatMetrics, FlatMetric{
						Name:               metric.Name(),
						Description:        metric.Description(),
						ScopeName:          scopeMetrics.Scope().Name(),
						ScopeVersion:       scopeMetrics.Scope().Version(),
						ResourceAttributes: attributeToMap(resourceMetrics.Resource().Attributes()),
						ScopeAttributes:    attributeToMap(scopeMetrics.Scope().Attributes()),
						MetricAttributes:   attributeToMap(dataPointsAttributes[l]),
						Type:               metric.Type().String(),
					})
				}
			}
		}
	}

	return flatMetrics
}

// attributeToMap converts pdata.AttributeMap to a map using the string representation of the values.
func attributeToMap(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)
	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = v.AsString()
		return true
	})
	return attrMap
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
