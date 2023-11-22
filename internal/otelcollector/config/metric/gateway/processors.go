package gateway

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/system"
)

func makeProcessorsConfig() Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		K8sAttributes:      gatewayprocs.K8sAttributesProcessorConfig(),
		InsertClusterName:  gatewayprocs.InsertClusterNameProcessorConfig(),
		ResolveServiceName: makeResolveServiceNameConfig(),
		DropKymaAttributes: gatewayprocs.DropKymaAttributesProcessorConfig(),
	}
}

func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "0.1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 20,
	}
}

func makeDropIfInputSourceRuntimeConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceRuntime),
			},
		},
	}
}

func makeDropIfInputSourcePrometheusConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourcePrometheus),
			},
		},
	}
}

func makeDropIfInputSourceIstioConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceIstio),
			},
		},
	}
}

func makeResolveServiceNameConfig() *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:        "ignore",
		MetricStatements: gatewayprocs.ResolveServiceNameStatements(),
	}
}

func makeFilterByNamespaceRuntimeInputConfig(namespaceSelector v1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, inputSourceEquals(metric.InputSourceRuntime))
}

func makeFilterByNamespacePrometheusInputConfig(namespaceSelector v1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, inputSourceEquals(metric.InputSourcePrometheus))
}

func makeFilterByNamespaceIstioInputConfig(namespaceSelector v1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, inputSourceEquals(metric.InputSourceIstio))
}

func makeFilterByNamespaceOtlpInputConfig(namespaceSelector v1alpha1.MetricPipelineInputNamespaceSelector) *FilterProcessor {
	return makeFilterByNamespaceConfig(namespaceSelector, otlpInputSource())
}

func makeFilterByNamespaceConfig(namespaceSelector v1alpha1.MetricPipelineInputNamespaceSelector, inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := createNamespacesConditions(namespaceSelector.Exclude)
		excludeNamespacesExpr := config.JoinWithAnd(inputSourceCondition, config.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := createNamespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := not(config.JoinWithAnd(inputSourceCondition, config.JoinWithOr(namespacesConditions...)))
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	if !*namespaceSelector.System {
		namespacesConditions := createNamespacesConditions(system.Namespaces())
		systemNamespacesExpr := config.JoinWithAnd(inputSourceCondition, config.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, systemNamespacesExpr)
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
		namespacesConditions = append(namespacesConditions, config.NamespaceEquals(ns))
	}
	return namespacesConditions
}

func inputSourceEquals(inputSourceType metric.InputSourceType) string {
	return config.ResourceAttributeEquals(metric.InputSourceAttribute, string(inputSourceType))
}

func otlpInputSource() string {
	// kyma.source attribute is only set by the metric agents for runtime, prometheus and istio metrics
	// Thus, kyma.source attribute will be nil for push-based otlp metrics
	return fmt.Sprintf("resource.attributes[\"%s\"] == nil", metric.InputSourceAttribute)
}

func not(expression string) string {
	return fmt.Sprintf("not(%s)", expression)
}
