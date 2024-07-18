package metric

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

type FlatMetric struct {
	Name, Description                                     string
	ResourceAttributes, ScopeAttributes, MetricAttributes map[string]string
	Type                                                  pmetric.MetricType
	ScopeAndVersion                                       ScopeVersion
}

type ScopeVersion struct {
	Name, Version string
}

func unmarshalMetrics(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
	return matchers.UnmarshalSignals[pmetric.Metrics](jsonlMetrics, func(buf []byte) (pmetric.Metrics, error) {
		var unmarshaler pmetric.JSONUnmarshaler
		return unmarshaler.UnmarshalMetrics(buf)
	})
}

func flattenAllMetrics(mds []pmetric.Metrics) []FlatMetric {
	var flatMetrics []FlatMetric

	for _, md := range mds {
		flatMetrics = append(flatMetrics, flattenMetrics(md)...)
	}

	return flatMetrics

}

func flattenMetrics(md pmetric.Metrics) []FlatMetric {
	var flatMetrics []FlatMetric

	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics := resourceMetrics.ScopeMetrics().At(j)
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)
				flatMetrics = append(flatMetrics, FlatMetric{
					Name:               metric.Name(),
					Description:        metric.Description(),
					ScopeAndVersion:    ScopeVersion{scopeMetrics.Scope().Name(), scopeMetrics.Scope().Version()},
					ResourceAttributes: attributeToMap(resourceMetrics.Resource().Attributes()),
					ScopeAttributes:    attributeToMap(scopeMetrics.Scope().Attributes()),
					MetricAttributes:   attributeToMap(getAttributesPerDataPoint(metric)[0]),
					Type:               metric.Type(),
				})
			}
		}
	}

	return flatMetrics

}

func attributeToMap(attrs pcommon.Map) map[string]string {
	attrMap := make(map[string]string)
	attrs.Range(func(k string, v pcommon.Value) bool {
		attrMap[k] = v.AsString()
		return true
	})
	return attrMap
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

func getScope(md pmetric.Metrics) []pmetric.ScopeMetrics {
	var scopeMetrics []pmetric.ScopeMetrics
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		resourceMetrics := md.ResourceMetrics().At(i)
		for j := 0; j < resourceMetrics.ScopeMetrics().Len(); j++ {
			scopeMetrics = append(scopeMetrics, resourceMetrics.ScopeMetrics().At(j))
		}
	}
	return scopeMetrics
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
