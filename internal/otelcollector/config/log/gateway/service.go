package gateway

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

func makePipelineServiceConfig(pipeline *telemetryv1alpha1.LogPipeline) config.Pipeline {
	processorIDs := []string{"memory_limiter"}

	if pipeline.Spec.Input.OTLP != nil && shouldFilterByNamespace(pipeline.Spec.Input.OTLP.Namespaces) {
		processorIDs = append(processorIDs, formatNamespaceFilterID(pipeline.Name))
	}

	processorIDs = append(processorIDs,
		"transform/set-observed-time-if-zero",
		"k8sattributes",
		"resource/insert-cluster-attributes",
		"service_enrichment",
		"resource/drop-kyma-attributes",
		"batch",
	)

	return config.Pipeline{
		Receivers:  []string{"otlp"},
		Processors: processorIDs,
		Exporters:  []string{formatOTLPExporterID(pipeline)},
	}
}

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func formatNamespaceFilterID(pipelineName string) string {
	return fmt.Sprintf("filter/%s-filter-by-namespace", pipelineName)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.LogPipeline) string {
	return otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}
