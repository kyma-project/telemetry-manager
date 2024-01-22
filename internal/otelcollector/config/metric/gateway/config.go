package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type Config struct {
	config.Base `yaml:",inline"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	OTLP config.OTLPReceiver `yaml:"otlp"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	K8sAttributes                                *config.K8sAttributesProcessor `yaml:"k8sattributes,omitempty"`
	InsertClusterName                            *config.ResourceProcessor      `yaml:"resource/insert-cluster-name,omitempty"`
	DropDiagnosticMetricsIfInputSourcePrometheus *FilterProcessor               `yaml:"filter/drop-diagnostic-metrics-if-input-source-prometheus,omitempty"`
	DropDiagnosticMetricsIfInputSourceIstio      *FilterProcessor               `yaml:"filter/drop-diagnostic-metrics-if-input-source-istio,omitempty"`
	DropIfInputSourceRuntime                     *FilterProcessor               `yaml:"filter/drop-if-input-source-runtime,omitempty"`
	DropIfInputSourcePrometheus                  *FilterProcessor               `yaml:"filter/drop-if-input-source-prometheus,omitempty"`
	DropIfInputSourceIstio                       *FilterProcessor               `yaml:"filter/drop-if-input-source-istio,omitempty"`
	DropIfInputSourceOtlp                        *FilterProcessor               `yaml:"filter/drop-if-input-source-otlp,omitempty"`
	ResolveServiceName                           *TransformProcessor            `yaml:"transform/resolve-service-name,omitempty"`
	DropKymaAttributes                           *config.ResourceProcessor      `yaml:"resource/drop-kyma-attributes,omitempty"`

	// NamespaceFilters contains filter processors, which need different configurations per pipeline
	NamespaceFilters NamespaceFilters `yaml:",inline,omitempty"`
}

type NamespaceFilters map[string]*FilterProcessor

type FilterProcessor struct {
	Metrics FilterProcessorMetrics `yaml:"metrics"`
}

type FilterProcessorMetrics struct {
	DataPoint []string `yaml:"datapoint,omitempty"`
	Metric    []string `yaml:"metric,omitempty"`
}

type TransformProcessor struct {
	ErrorMode        string                                `yaml:"error_mode"`
	MetricStatements []config.TransformProcessorStatements `yaml:"metric_statements"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
