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

func (e Exporter) ExportGaugeMetrics(ctx context.Context, pmetrics pmetric.Metrics) error {
	return e.otlpExporter.Export(ctx, toResourceMetrics(pmetrics, getGaugeMetricDataPoints, convertGaugeMetric))
}

func (e Exporter) ExportSumMetrics(ctx context.Context, pmetrics pmetric.Metrics) error {
	return e.otlpExporter.Export(ctx, toResourceMetrics(pmetrics, getSumMetricDataPoints, convertSumMetric))
}

func convertGaugeMetric(metric pmetric.Metric, dataPoints []metricdata.DataPoint[float64]) metricdata.Metrics {
	return metricdata.Metrics{
		Name:        metric.Name(),
		Description: metric.Description(),
		Unit:        metric.Unit(),
		Data: metricdata.Gauge[float64]{
			DataPoints: dataPoints,
		},
	}
}

func convertSumMetric(metric pmetric.Metric, dataPoints []metricdata.DataPoint[float64]) metricdata.Metrics {
	temporality := metricdata.DeltaTemporality

	if metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
		temporality = metricdata.CumulativeTemporality
	}

	return metricdata.Metrics{
		Name:        metric.Name(),
		Description: metric.Description(),
		Unit:        metric.Unit(),
		Data: metricdata.Sum[float64]{
			DataPoints:  dataPoints,
			Temporality: temporality,
		},
	}
}

func getSumMetricDataPoints(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	return metric.Sum().DataPoints()
}

func getGaugeMetricDataPoints(metric pmetric.Metric) pmetric.NumberDataPointSlice {
	return metric.Gauge().DataPoints()
}

// toResourceMetrics converts metrics from pmetric.Metrics to metricdata.ResourceMetrics.
func toResourceMetrics(pmetrics pmetric.Metrics, retrieveDataPoints DataPointsRetrieval, convertMetrics MetricConvertion) *metricdata.ResourceMetrics {
	var scopeMetrics []metricdata.Metrics

	for i := 0; i < pmetrics.ResourceMetrics().Len(); i++ {
		res := pmetrics.ResourceMetrics().At(i)
		for j := 0; j < res.ScopeMetrics().Len(); j++ {
			sc := res.ScopeMetrics().At(j)
			for k := 0; k < sc.Metrics().Len(); k++ {
				metrics := sc.Metrics().At(k)

				var dataPoints []metricdata.DataPoint[float64]
				for l := 0; l < retrieveDataPoints(metrics).Len(); l++ {
					d := retrieveDataPoints(metrics).At(l)

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

				scopeMetrics = append(scopeMetrics, convertMetrics(metrics, dataPoints))
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
