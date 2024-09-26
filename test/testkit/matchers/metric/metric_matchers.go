package metric

import (
	"fmt"
	"maps"
	"slices"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

// HaveFlatMetrics extracts FlatMetrics from JSON and applies the matcher to them.
func HaveFlatMetrics(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonMetrics []byte) ([]FlatMetric, error) {
		mds, err := unmarshalMetrics(jsonMetrics)
		if err != nil {
			return nil, fmt.Errorf("WithMds requires a valid OTLP JSON document: %w", err)
		}

		fm := flattenAllMetrics(mds)

		return fm, nil
	}, matcher)
}

// HaveUniqueNames extracts metric names from all FlatMetrics and applies the matcher to them.
func HaveUniqueNames(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm []FlatMetric) []string {
		names := make(map[string]struct{})
		for _, m := range fm {
			names[m.Name] = struct{}{}
		}
		return slices.Sorted(maps.Keys(names))
	}, matcher)
}

// HaveUniqueNamesForRuntimeScope extracts metric names from FlatMetrics with runtime scope and applies the matcher to them.
func HaveUniqueNamesForRuntimeScope(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm []FlatMetric) []string {
		names := make(map[string]struct{})
		for _, m := range fm {
			if m.ScopeName != metric.InstrumentationScopeRuntime {
				continue
			}
			names[m.Name] = struct{}{}
		}
		return slices.Sorted(maps.Keys(names))
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

// HaveKeys extracts keys from a map[string][string] and applies the matcher to them.
func HaveKeys(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m map[string]string) []string {
		return slices.Sorted(maps.Keys(m))
	}, matcher)
}
