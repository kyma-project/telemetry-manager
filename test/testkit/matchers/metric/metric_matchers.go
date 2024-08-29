package metric

import (
	"fmt"
	"slices"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
)

// WithFlatMetricsDataPoints extracts FlatMetricDataPoints from JSON and applies the matcher to them.
func WithFlatMetricsDataPoints(matcher types.GomegaMatcher) types.GomegaMatcher {
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

// WithName extracts name from FlatMetricDataPoint and applies the matcher to it.
func WithName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.Name
	}, matcher)
}

// WithScopeName extracts scope name from FlatMetricDataPoint and applies the matcher to it.
func WithScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.ScopeName
	}, matcher)
}

// WithScopeVersion extracts scope version from FlatMetricDataPoint and applies the matcher to it.
func WithScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.ScopeVersion
	}, matcher)
}

// WithResourceAttributes extracts resource attributes from FlatMetricDataPoint and applies the matcher to them.
func WithResourceAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) map[string]string {
		return fm.ResourceAttributes
	}, matcher)
}

// WithMetricAttributes extracts metric attributes from FlatMetricDataPoint and applies the matcher to them.
func WithMetricAttributes(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) map[string]string {
		return fm.MetricAttributes
	}, matcher)
}

// WithType extracts type from FlatMetricDataPoint and applies the matcher to it.
func WithType(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm FlatMetricDataPoint) string {
		return fm.Type
	}, matcher)
}

// WithKeys extracts key from a map[string][string] and applies the matcher to it.
func WithKeys(matcher types.GomegaMatcher) types.GomegaMatcher {
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
