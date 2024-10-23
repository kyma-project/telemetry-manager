package gateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

func makeProcessorsConfig() Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		K8sAttributes:                 gatewayprocs.K8sAttributesProcessorConfig(),
		InsertClusterName:             gatewayprocs.InsertClusterNameProcessorConfig(),
		ResolveServiceName:            makeResolveServiceNameConfig(),
		DropKymaAttributes:            gatewayprocs.DropKymaAttributesProcessorConfig(),
		DeleteSkipEnrichmentAttribute: makeDeleteSkipEnrichmentAttributeConfig(),
	}
}

//nolint:mnd // hardcoded values
func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

//nolint:mnd // hardcoded values
func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}

func makeResolveServiceNameConfig() *metric.TransformProcessor {
	return &metric.TransformProcessor{
		ErrorMode:        "ignore",
		MetricStatements: gatewayprocs.ResolveServiceNameStatements(),
	}
}

func makeDeleteSkipEnrichmentAttributeConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "delete",
				Key:    metric.SkipEnrichmentAttribute,
			},
		},
	}
}

func makeDropIfInputSourceRuntimeConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.ScopeNameEquals(metric.InstrumentationScopeRuntime),
			},
		},
	}
}

func makeDropIfInputSourcePrometheusConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.ScopeNameEquals(metric.InstrumentationScopePrometheus),
			},
		},
	}
}

func makeDropIfInputSourceIstioConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio),
			},
		},
	}
}

func makeDropIfInputSourceOtlpConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				otlpInputSource(),
			},
		},
	}
}

func makeDropRuntimePodMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.pod.*"),
				),
			},
		},
	}
}

func makeDropRuntimeContainerMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "(^k8s.container.*)|(^container.*)"),
				),
			},
		},
	}
}

func makeDropRuntimeNodeMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.node.*"),
				),
			},
		},
	}
}

func makeDropRuntimeVolumeMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.volume.*"),
				),
			},
		},
	}
}

func makeFilterByNamespaceRuntimeInputConfig(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, inputSourceEquals(metric.InputSourceRuntime))
}

func makeFilterByNamespacePrometheusInputConfig(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, inputSourceEquals(metric.InputSourcePrometheus))
}

func makeFilterByNamespaceIstioInputConfig(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, inputSourceEquals(metric.InputSourceIstio))
}

func makeFilterByNamespaceOtlpInputConfig(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, otlpInputSource())
}

func makeFilterByNamespaceConfig(namespaceSelector *telemetryv1alpha1.MetricPipelineInputNamespaceSelector, inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := createNamespacesConditions(namespaceSelector.Exclude)
		excludeNamespacesExpr := ottlexpr.JoinWithAnd(inputSourceCondition, ottlexpr.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := createNamespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := ottlexpr.JoinWithAnd(inputSourceCondition, not(ottlexpr.JoinWithOr(namespacesConditions...)))
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

func createNamespacesConditions(namespaces []string) []string {
	var namespacesConditions []string
	for _, ns := range namespaces {
		namespacesConditions = append(namespacesConditions, ottlexpr.NamespaceEquals(ns))
	}

	return namespacesConditions
}

func inputSourceEquals(inputSourceType metric.InputSourceType) string {
	return ottlexpr.ScopeNameEquals(metric.InstrumentationScope[inputSourceType])
}

func otlpInputSource() string {
	// When instrumentation scope is not set to
	// io.kyma-project.telemetry/runtime or io.kyma-project.telemetry/prometheus or io.kyma-project.telemetry/istio
	// we assume the metric is being pushed directly to metrics gateway.
	return fmt.Sprintf("not(%s or %s or %s)",
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeRuntime),
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopePrometheus),
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio),
	)
}

func not(expression string) string {
	return fmt.Sprintf("not(%s)", expression)
}
