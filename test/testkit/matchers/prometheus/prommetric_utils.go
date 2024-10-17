package prometheus

import prommodel "github.com/prometheus/client_model/go"

type FlatMetricFamily struct {
	Name         string
	MetricValues float64
	Labels       map[string]string
}

func flattenAllMetricFamily(mfs map[string]*prommodel.MetricFamily) []FlatMetricFamily {
	var fmf []FlatMetricFamily
	for _, mf := range mfs {
		fmf = append(fmf, flattenMetricFamily(mf)...)
	}
	return fmf
}

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
