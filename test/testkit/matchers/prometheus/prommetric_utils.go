package prometheus

import prommodel "github.com/prometheus/client_model/go"

// FlatMetricFamily holds all necessary information about a prometheus MetricFamily.
// Gomega doesn't handle deeply nested data structure very well and generates large, unreadable diffs when paired with
// the deeply nested structure of MetricFamily.
//
// FlatMetricFamily is a flat data structure that provides necessary information from different levels of
// MetricFamily, making accessing the information easier and improves readability of the Gomega output.
type FlatMetricFamily struct {
	Name        string
	MetricType  string
	MetricValue float64
	Labels      map[string]string
}

// flattenAllMetricFamilies flattens an array of prometheus MetricFamily to a slice of FlatMetricFamily.
// It converts the deeply nested MetricFamily to a flat struct, making it more readable in the test output.
func flattenAllMetricFamilies(mfs map[string]*prommodel.MetricFamily) []FlatMetricFamily {
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
		t, v := getTypeAndValuePerMetric(m)
		fmf = append(fmf, FlatMetricFamily{
			Name:        mf.GetName(),
			MetricType:  t,
			MetricValue: v,
			Labels:      labelsToMap(m.GetLabel()),
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

const (
	gaugeType   = "Gauge"
	counterType = "Counter"
	untypedType = "Untyped"
)

func getTypeAndValuePerMetric(m *prommodel.Metric) (string, float64) {
	if m.Gauge != nil {
		return gaugeType, m.Gauge.GetValue()
	}

	if m.Counter != nil {
		return counterType, m.Counter.GetValue()
	}

	if m.Untyped != nil {
		return untypedType, m.Untyped.GetValue()
	}

	return "", 0
}
