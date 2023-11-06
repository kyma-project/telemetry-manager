package metrics

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

type Exporter struct {
	otlpExporter metric.Exporter
}

type MetricConvertion = func(metric pmetric.Metric, dataPoints []metricdata.DataPoint[float64]) metricdata.Metrics

type DataPointsRetrieval = func(metric pmetric.Metric) pmetric.NumberDataPointSlice

// NewExporter is an adapter over the OTLP metric.Exporter instance.
func NewExporter(e metric.Exporter) Exporter {
	return Exporter{otlpExporter: e}
}

func (e Exporter) ExportMetrics(ctx context.Context, pmetrics pmetric.Metrics) error {
	return e.otlpExporter.Export(ctx, toResourceMetrics(pmetrics))
}

// toResourceMetrics converts metrics from pmetric.Metrics to metricdata.ResourceMetrics.
func toResourceMetrics(pmetrics pmetric.Metrics) *metricdata.ResourceMetrics {
	var scopeMetrics []metricdata.Metrics

	for i := 0; i < pmetrics.ResourceMetrics().Len(); i++ {
		res := pmetrics.ResourceMetrics().At(i)
		for j := 0; j < res.ScopeMetrics().Len(); j++ {
			sc := res.ScopeMetrics().At(j)
			for k := 0; k < sc.Metrics().Len(); k++ {
				metrics := sc.Metrics().At(k)

				var dataPoints []metricdata.DataPoint[float64]
				for l := 0; l < metrics.Gauge().DataPoints().Len(); l++ {
					d := metrics.Gauge().DataPoints().At(l)

					var attrs []attribute.KeyValue
					for k, v := range d.Attributes().AsRaw() {
						attrs = append(attrs, attribute.String(k, v.(string)))
					}

					dataPoints = append(dataPoints, metricdata.DataPoint[float64]{
						Attributes: attribute.NewSet(attrs...),
						StartTime:  d.StartTimestamp().AsTime(),
						Time:       d.Timestamp().AsTime(),
						Value:      d.DoubleValue(),
					})
				}

				scopeMetrics = append(scopeMetrics, metricdata.Metrics{
					Name:        metrics.Name(),
					Description: metrics.Description(),
					Unit:        metrics.Unit(),
					Data: metricdata.Gauge[float64]{
						DataPoints: dataPoints,
					},
				})
			}
		}
	}

	return &metricdata.ResourceMetrics{
		Resource: resource.NewSchemaless(),
		ScopeMetrics: []metricdata.ScopeMetrics{{
			Metrics: scopeMetrics,
		}},
	}
}
