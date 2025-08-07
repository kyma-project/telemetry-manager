package gateway

import (
	"context"
	"fmt"
	"maps"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

// newConfig constructs a global, pipeline-independent Base config for the metric agent collector.
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

// declareComponentsForTracePipeline enriches a Config (exporters, processors, etc.) with components for a given telemetryv1alpha1.TracePipeline.
func (cfg *Config) addTracePipelineComponents(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.TracePipeline, envVars otlpexporter.EnvVars) error {
	return cfg.addOTLPExporter(ctx, otlpExporterBuilder, pipeline, envVars)
}

func (cfg *Config) addOTLPExporter(ctx context.Context, otlpExporterBuilder *otlpexporter.ConfigBuilder, pipeline *telemetryv1alpha1.TracePipeline, envVars otlpexporter.EnvVars) error {
	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.MakeConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to make otlp exporter config: %w", err)
	}

	maps.Copy(envVars, otlpExporterEnvVars)

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
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

	K8sAttributes           *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	IstioNoiseFilter        *config.IstioNoiseFilterProcessor  `yaml:"istio_noise_filter,omitempty"`
	ResolveServiceName      *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes      *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}
