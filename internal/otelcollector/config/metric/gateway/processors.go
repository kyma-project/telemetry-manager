package gateway

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/system"
	"strings"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
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

func makeFilterByNamespaceConfig(namespaceSelector v1alpha1.MetricPipelineInputNamespaceSelector, inputSourceCondition string) *FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := createNamespacesConditions(namespaceSelector.Exclude)
		excludeNamespacesExpr := joinWithAnd(inputSourceCondition, joinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := createNamespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := not(joinWithAnd(inputSourceCondition, joinWithOr(namespacesConditions...)))
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	if !*namespaceSelector.System {
		namespacesConditions := createNamespacesConditions(system.Namespaces())
		systemNamespacesExpr := joinWithAnd(inputSourceCondition, joinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, systemNamespacesExpr)
	}

	//	fmt.Sprintf("not (%s and %s)", inputSourceEquals(metric.InputSourceRuntime), namespaceEquals("foo")), //WORKS!!!
	// fmt.Sprintf("%s and %s", inputSourceEquals(""), namespaceEquals("foo")), // didn't work - metrics were not dropped
	// fmt.Sprintf("%s", namespaceEquals("foo")), //works
	// fmt.Sprintf(OtlpInputSource()), // works
	// fmt.Sprintf("%s and %s", OtlpInputSource(), joinWithOr(namespaceEquals("foo"), namespaceEquals("bar"))),
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: filterExpressions,
		},
	}
}

func createNamespacesConditions(namespaces []string) []string {
	var namespacesConditions []string
	for _, ns := range namespaces {
		namespacesConditions = append(namespacesConditions, namespaceEquals(ns))
	}
	return namespacesConditions
}

func inputSourceEquals(inputSourceType metric.InputSourceType) string {
	return resourceAttributeEquals(metric.InputSourceAttribute, string(inputSourceType))
}

func OtlpInputSource() string {
	return "resource.attributes[\"" + metric.InputSourceAttribute + "\"] == nil"
}

func namespaceEquals(name string) string {
	return resourceAttributeEquals("k8s.namespace.name", name)
}

func resourceAttributeEquals(key, value string) string {
	return "resource.attributes[\"" + key + "\"] == \"" + value + "\""
}

func joinWithOr(parts ...string) string {
	return "(" + strings.Join(parts, " or ") + ")"
}

func joinWithAnd(parts ...string) string {
	return strings.Join(parts, " and ")
}

func not(expression string) string {
	return fmt.Sprintf("not (%s)", expression)
}
