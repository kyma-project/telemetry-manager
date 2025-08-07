package gateway

import (
	"context"
	"fmt"
	"maps"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

// newConfig constructs a global, pipeline-independent Base config for the log gateway collector.
// It sets up default collector components, and returns a Config with initialized fields.
func newConfig(opts BuildOptions) *Config {
	return &Config{
		Base: config.Base{
			Service:    config.DefaultService(make(config.Pipelines)),
			Extensions: config.DefaultExtensions(),
		},
		Receivers:  receiversConfig(),
		Processors: processorsConfig(opts),
		Exporters:  make(Exporters),
	}
}

// addLogPipelineComponents enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.LogPipeline.
func (cfg *Config) addLogPipelineComponents(
	ctx context.Context,
	otlpExporterBuilder *otlpexporter.ConfigBuilder,
	pipeline *telemetryv1alpha1.LogPipeline,
	envVars otlpexporter.EnvVars,
) error {
	cfg.addNamespaceFilter(pipeline)
	cfg.addInputSourceFilters(pipeline)

	return cfg.addOTLPExporter(ctx, otlpExporterBuilder, pipeline, envVars)
}

func (cfg *Config) addNamespaceFilter(pipeline *telemetryv1alpha1.LogPipeline) {
	otlpInput := pipeline.Spec.Input.OTLP
	if otlpInput == nil || otlpInput.Disabled {
		// no namespace filter needed
		return
	}

	if cfg.Processors.NamespaceFilters == nil {
		cfg.Processors.NamespaceFilters = make(NamespaceFilters)
	}

	if shouldFilterByNamespace(otlpInput.Namespaces) {
		processorID := formatNamespaceFilterID(pipeline.Name)
		cfg.Processors.NamespaceFilters[processorID] = namespaceFilterProcessorConfig(otlpInput.Namespaces)
	}
}

func (cfg *Config) addInputSourceFilters(pipeline *telemetryv1alpha1.LogPipeline) {
	input := pipeline.Spec.Input
	if !logpipelineutils.IsOTLPInputEnabled(input) {
		cfg.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
	}
}

func (cfg *Config) addOTLPExporter(
	ctx context.Context,
	otlpExporterBuilder *otlpexporter.ConfigBuilder,
	pipeline *telemetryv1alpha1.LogPipeline,
	envVars otlpexporter.EnvVars,
) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := formatOTLPExporterID(pipeline)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

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

	SetObsTimeIfZero        *log.TransformProcessor            `yaml:"transform/set-observed-time-if-zero,omitempty"`
	K8sAttributes           *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	IstioNoiseFilter        *config.IstioNoiseFilterProcessor  `yaml:"istio_noise_filter,omitempty"`
	InsertClusterAttributes *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	ResolveServiceName      *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes      *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
	DropIfInputSourceOTLP   *FilterProcessor                   `yaml:"filter/drop-if-input-source-otlp,omitempty"`
	IstioEnrichment         *IstioEnrichmentProcessor          `yaml:"istio_enrichment,omitempty"`

	// NamespaceFilters contains filter processors, which need different configurations per pipeline
	NamespaceFilters NamespaceFilters `yaml:",inline,omitempty"`
}

type NamespaceFilters map[string]*FilterProcessor

type FilterProcessor struct {
	Logs FilterProcessorLogs `yaml:"logs"`
}

type FilterProcessorLogs struct {
	Log []string `yaml:"log_record,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}

type IstioEnrichmentProcessor struct {
	ScopeVersion string `yaml:"scope_version,omitempty"`
}
