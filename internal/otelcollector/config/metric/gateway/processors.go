package gateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
)

func makeProcessorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		K8sAttributes:                 processors.K8sAttributesProcessorConfig(processors.Enrichments{}),
		InsertClusterAttributes:       processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.CloudProvider),
		ResolveServiceName:            processors.MakeResolveServiceNameConfig(),
		DropKymaAttributes:            processors.DropKymaAttributesProcessorConfig(),
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
				ottlexpr.ResourceAttributeEquals(metric.KymaInputNameAttribute, metric.KymaInputPrometheus),
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

func makeDropIfEnvoyMetricsDisabledConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(ottlexpr.IsMatch("name", "^envoy_.*"), ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio)),
			},
		},
	}
}

func makeDropIfInputSourceOTLPConfig() *FilterProcessor {
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

func makeDropRuntimeDeploymentMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.deployment.*"),
				),
			},
		},
	}
}

func makeDropRuntimeStatefulSetMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.statefulset.*"),
				),
			},
		},
	}
}

func makeDropRuntimeDaemonSetMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.daemonset.*"),
				),
			},
		},
	}
}

func makeDropRuntimeJobMetricsConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				ottlexpr.JoinWithAnd(
					inputSourceEquals(metric.InputSourceRuntime),
					ottlexpr.IsMatch("name", "^k8s.job.*"),
				),
			},
		},
	}
}

func makeFilterByNamespaceConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector, inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := makeNamespacesConditions(namespaceSelector.Exclude)

		// Drop metrics if the excluded namespaces are matched
		excludeNamespacesExpr := ottlexpr.JoinWithAnd(inputSourceCondition, ottlexpr.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := makeNamespacesConditions(namespaceSelector.Include)

		// metrics are dropped if the statement is true, so you need to negate the expression
		includeNamespacesExpr := ottlexpr.JoinWithAnd(
			// Ensure we are filtering metrics from the correct input source
			inputSourceCondition,

			// Ensure the k8s.namespace.name resource attribute is not nil,
			// so we don't drop logs without a namespace label
			ottlexpr.ResourceAttributeIsNotNil(ottlexpr.K8sNamespaceName),

			// Logs are dropped if the filter expression evaluates to true,
			// so we negate the match against included namespaces to keep only those
			ottlexpr.Not(ottlexpr.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

func makeNamespacesConditions(namespaces []string) []string {
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
	// When instrumentation scope is not set to any of the following values
	// io.kyma-project.telemetry/runtime, io.kyma-project.telemetry/prometheus, io.kyma-project.telemetry/istio, and io.kyma-project.telemetry/kyma
	// we assume the metric is being pushed directly to metrics gateway.
	return fmt.Sprintf("not(%s or %s or %s or %s)",
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeRuntime),
		ottlexpr.ResourceAttributeEquals(metric.KymaInputNameAttribute, metric.KymaInputPrometheus),
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeIstio),
		ottlexpr.ScopeNameEquals(metric.InstrumentationScopeKyma),
	)
}
