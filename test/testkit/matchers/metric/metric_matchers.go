package metric

import (
	"fmt"
	"slices"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// HaveFlatMetricsDataPoints extracts FlatMetricDataPoints from JSON and applies the matcher to them.
func HaveFlatMetricsDataPoints(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonMetrics []byte) ([]FlatMetricDataPoint, error) {
		mds, err := unmarshalMetrics(jsonMetrics)
		if err != nil {
			return nil, fmt.Errorf("WithMds requires a valid OTLP JSON document: %w", err)
		}

		fm := flattenAllMetricsDataPoints(mds)

		return fm, nil
	}, matcher)
}

// HaveUniqueNames extracts metric names from all FlatMetricDataPoints and applies the matcher to them.
// Rename to HaveUniqueNames
func HaveUniqueNames(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm []FlatMetricDataPoint) []string {
		var names []string
		for _, m := range fm {
			names = append(names, m.Name)
		}
		slices.Sort(names)
		return slices.Compact(names)
	}, matcher)
}

// HaveName extracts name from FlatMetricDataPoint and applies the matcher to it.
func HaveName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.Name
	}, matcher)
}

// HaveScopeName extracts scope name from FlatMetricDataPoint and applies the matcher to it.
func HaveScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.ScopeName
	}, matcher)
}

// HaveScopeVersion extracts scope version from FlatMetricDataPoint and applies the matcher to it.
func HaveScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.ScopeVersion
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatMetricDataPoint and applies the matcher to them.
func HaveResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) map[string]string {
		return fm.ResourceAttributes
	}, matcher)
}

// HaveMetricAttributes extracts metric attributes from FlatMetricDataPoint and applies the matcher to them.
func HaveMetricAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) map[string]string {
		return fm.MetricAttributes
	}, matcher)
}

// HaveType extracts type from FlatMetricDataPoint and applies the matcher to it.
func HaveType(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.Type
	}, matcher)
}

// HaveKeys extracts key from a map[string][string] and applies the matcher to it.
func HaveKeys(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m map[string]string) []string {
		keys := make([]string, len(m))
		i := 0
		for k := range m {
			keys[i] = k
			i++
		}
		return keys
	}, matcher)
}
