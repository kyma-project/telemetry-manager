package metric

import (
	"fmt"
	"slices"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// HaveFlatMetrics extracts FlatMetrics from JSON and applies the matcher to them.
func HaveFlatMetrics(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonMetrics []byte) ([]FlatMetric, error) {
		mds, err := unmarshalMetrics(jsonMetrics)
		if err != nil {
			return nil, fmt.Errorf("HaveFlatMetrics requires a valid OTLP JSON document: %w", err)
		}

		fm := flattenAllMetrics(mds)

		return fm, nil
	}, matcher)
}

// HaveUniqueNames extracts metric names from all FlatMetrics and applies the matcher to them.
// Rename to HaveUniqueNames
func HaveUniqueNames(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm []FlatMetric) []string {
		var names []string
		for _, m := range fm {
			names = append(names, m.Name)
		}
		slices.Sort(names)
		return slices.Compact(names)
	}, matcher)
}

// HaveName extracts name from FlatMetric and applies the matcher to it.
func HaveName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetric) string {
		return fm.Name
	}, matcher)
}

// HaveScopeName extracts scope name from FlatMetric and applies the matcher to it.
func HaveScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetric) string {
		return fm.ScopeName
	}, matcher)
}

// HaveScopeVersion extracts scope version from FlatMetric and applies the matcher to it.
func HaveScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetric) string {
		return fm.ScopeVersion
	}, matcher)
}

// HaveResourceAttributes extracts resource attributes from FlatMetric and applies the matcher to them.
func HaveResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetric) map[string]string {
		return fm.ResourceAttributes
	}, matcher)
}

// HaveMetricAttributes extracts metric attributes from FlatMetric and applies the matcher to them.
func HaveMetricAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetric) map[string]string {
		return fm.MetricAttributes
	}, matcher)
}

// HaveType extracts type from FlatMetric and applies the matcher to it.
func HaveType(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetric) string {
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
