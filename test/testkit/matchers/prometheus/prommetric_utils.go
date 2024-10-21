package prometheus

import prommodel "github.com/prometheus/client_model/go"

// FlatMetricFamily holds all necessary information about a prometheus MetricFamily.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with
// the deeply nested structure of pmetrics.
//
// Introducing a go struct with a flat data structure by extracting necessary information from different levels of
// metricfamily makes accessing the information easier and improves readability of the output.
type FlatMetricFamily struct {
	Name         string
	MetricValues float64
	Labels       map[string]string
}

// flattenAllMetricFamily flattens an array of prometheus MetricFamily to a slice of FlatMetricFamily.
// It converts the deeply nested MetricFamily to a flat struct, making it more readable in the test output.
func flattenAllMetricFamily(mfs map[string]*prommodel.MetricFamily) []FlatMetricFamily {
	var fmf []FlatMetricFamily
	for _, mf := range mfs {
		fmf = append(fmf, flattenMetricFamily(mf)...)
	}

	return fmf
}

// flattenMetricFamily converts a single MetricFamily into a slice of FlatMetricFamily
// It loops through all the metrics in a MetricFamily and appends it to the FlatMetricFamily slice.
func flattenMetricFamily(mf *prommodel.MetricFamily) []FlatMetricFamily {
	var fmf []FlatMetricFamily

	for _, m := range mf.Metric {
		v := getValuePerMetric(m)
		fmf = append(fmf, FlatMetricFamily{
			Name:         mf.GetName(),
			MetricValues: v,
			Labels:       labelsToMap(m.GetLabel()),
		})
	}

	return fmf
}

func labelsToMap(l []*prommodel.LabelPair) map[string]string {
	labels := make(map[string]string)
	for _, l := range l {
		labels[l.GetName()] = l.GetValue()
	}

	return labels
}

func getValuePerMetric(m *prommodel.Metric) float64 {
	if m.Gauge != nil {
		return m.Gauge.GetValue()
	}

	if m.Counter != nil {
		return m.Counter.GetValue()
	}

	if m.Untyped != nil {
		return m.Untyped.GetValue()
	}

	return 0
}
