package metric

import (
	"fmt"
	"slices"
	"strings"

	"github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func WithMds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]pmetric.Metrics, error) {
		mds, err := unmarshalMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("WithMds requires a valid OTLP JSON document: %w", err)
		}

		return mds, nil
	}, matcher)
}

// WithNames extracts metric names from FlatMetrics and applies the matcher to them.
func WithNames(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm []FlatMetricDataPoint) ([]string, error) {
		var names []string
		for _, m := range fm {
			names = append(names, m.Name)
		}
		slices.Sort(names)
		return slices.Compact(names), nil
	}, matcher)
}

// WithScopeAndVersion extracts scope and version from FlatMetrics and applies the matcher to them.
func WithScopeAndVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(fm []FlatMetricDataPoint) ([]ScopeVersion, error) {
		var scopes []ScopeVersion
		for _, m := range fm {
			scopes = append(scopes, m.ScopeAndVersion)
		}
		slices.SortFunc(scopes, func(i, j ScopeVersion) int {
			if i.Name == j.Name {
				return strings.Compare(i.Version, j.Version)
			}
			return strings.Compare(i.Name, j.Name)
		})
		return slices.Compact(scopes), nil
	}, matcher)
}

// WithFlatMetrics extracts FlatMetrics from OTLP JSON and applies the matcher to them.
func WithFlatMetrics(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(jsonlMetrics []byte) ([]FlatMetricDataPoint, error) {
		mds, err := unmarshalMetrics(jsonlMetrics)
		if err != nil {
			return nil, fmt.Errorf("WithMds requires a valid OTLP JSON document: %w", err)
		}

		fm := flattenAllMetrics(mds)

		return fm, nil
	}, matcher)
}

// ContainMd is an alias for WithMds(gomega.ContainElement()).
func ContainMd(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithMds(gomega.ContainElement(matcher))
}

// ConsistOfMds is an alias for WithMds(gomega.ConsistOf()).
func ConsistOfMds(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithMds(gomega.ConsistOf(matcher))
}

func WithMetrics(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(md pmetric.Metrics) ([]pmetric.Metric, error) {
		return getMetrics(md), nil
	}, matcher)
}

func WithScope(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(md pmetric.Metrics) ([]pmetric.ScopeMetrics, error) {
		return getScope(md), nil
	}, matcher)
}
func ContainScope(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithScope(gomega.ContainElement(matcher))
}

// ContainMetric is an alias for WithMetrics(gomega.ContainElement()).
func ContainMetric(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithMetrics(gomega.ContainElement(matcher))
}

func WithResourceAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(md pmetric.Metrics) ([]map[string]any, error) {
		var rawAttrs []map[string]any
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			rawAttrs = append(rawAttrs, md.ResourceMetrics().At(i).Resource().Attributes().AsRaw())
		}
		return rawAttrs, nil
	}, matcher)
}

// ContainResourceAttrs is an alias for WithResourceAttrs(gomega.ContainElement()).
func ContainResourceAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithResourceAttrs(gomega.ContainElement(matcher))
}

func WithScopeName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(sm pmetric.ScopeMetrics) (string, error) {
		return sm.Scope().Name(), nil
	}, matcher)
}

func WithScopeVersion(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(sm pmetric.ScopeMetrics) (string, error) {
		return sm.Scope().Version(), nil
	}, matcher)
}

func WithName(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m pmetric.Metric) (string, error) {
		return m.Name(), nil
	}, matcher)
}

func WithType(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m pmetric.Metric) (pmetric.MetricType, error) {
		return m.Type(), nil
	}, matcher)
}

func WithDataPointAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return gomega.WithTransform(func(m pmetric.Metric) ([]map[string]any, error) {
		var rawAttrs []map[string]any
		for _, attrs := range getAttributesPerDataPoint(m) {
			rawAttrs = append(rawAttrs, attrs.AsRaw())
		}
		return rawAttrs, nil
	}, matcher)
}

// ContainDataPointAttrs is an alias for WithDataPointAttrs(gomega.ContainElement()).
func ContainDataPointAttrs(matcher types.GomegaMatcher) types.GomegaMatcher {
	return WithDataPointAttrs(gomega.ContainElement(matcher))
}
