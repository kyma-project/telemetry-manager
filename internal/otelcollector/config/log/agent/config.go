package agent

import (
	"context"
	"fmt"
	"maps"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/otlpexporter"
)

// newConfig constructs a global, pipeline-independent Base config for the log gateway collector.
// It sets up default collector components, and returns a Config with initialized fields.
func newConfig(opts BuildOptions) *Config {
	service := config.DefaultService(make(config.Pipelines))
	service.Extensions = append(service.Extensions, "file_storage")

	return &Config{
		Service:    service,
		Extensions: extensionsConfig(),
		Receivers:  make(Receivers),
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
	cfg.addFileLogReceiver(pipeline)
	return cfg.addOTLPExporter(ctx, otlpExporterBuilder, pipeline, envVars)
}

func (cfg *Config) addFileLogReceiver(pipeline *telemetryv1alpha1.LogPipeline) {
	receiver := fileLogReceiverConfig(*pipeline)
	otlpReceiverID := fmt.Sprintf("filelog/%s", pipeline.Name)
	cfg.Receivers[otlpReceiverID] = Receiver{FileLog: receiver}
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

	otlpExporterID := otlpexporter.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
	cfg.Exporters[otlpExporterID] = Exporter{OTLP: otlpExporterConfig}

	return nil
}

type Config struct {
	Service    config.Service `yaml:"service"`
	Extensions Extensions     `yaml:"extensions"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers map[string]Receiver

type Receiver struct {
	FileLog *FileLog `yaml:",inline,omitempty"`
}

type FileLog struct {
	Exclude         []string              `yaml:"exclude,omitempty"`
	Include         []string              `yaml:"include,omitempty"`
	IncludeFileName *bool                 `yaml:"include_file_name,omitempty"`
	IncludeFilePath *bool                 `yaml:"include_file_path,omitempty"`
	StartAt         string                `yaml:"start_at,omitempty"`
	Storage         string                `yaml:"storage,omitempty"`
	RetryOnFailure  config.RetryOnFailure `yaml:"retry_on_failure,omitempty"`
	Operators       []Operator            `yaml:"operators,omitempty"`
}

type Operator struct {
	ID                      string            `yaml:"id,omitempty"`
	Type                    OperatorType      `yaml:"type,omitempty"`
	AddMetadataFromFilePath *bool             `yaml:"add_metadata_from_file_path,omitempty"`
	Format                  string            `yaml:"format,omitempty"`
	From                    string            `yaml:"from,omitempty"`
	To                      string            `yaml:"to,omitempty"`
	IfExpr                  string            `yaml:"if,omitempty"`
	ParseFrom               string            `yaml:"parse_from,omitempty"`
	ParseTo                 string            `yaml:"parse_to,omitempty"`
	Field                   string            `yaml:"field,omitempty"`
	TraceID                 OperatorAttribute `yaml:"trace_id,omitempty"`
	SpanID                  OperatorAttribute `yaml:"span_id,omitempty"`
	TraceFlags              OperatorAttribute `yaml:"trace_flags,omitempty"`
	Regex                   string            `yaml:"regex,omitempty"`
	Trace                   TraceAttribute    `yaml:"trace,omitempty"`
	Routes                  []Route           `yaml:"routes,omitempty"`
	Default                 string            `yaml:"default,omitempty"`
	Output                  string            `yaml:"output,omitempty"`
}

type OperatorType string

const (
	Move           OperatorType = "move"
	SeverityParser OperatorType = "severity_parser"
	RegexParser    OperatorType = "regex_parser"
	Remove         OperatorType = "remove"
	Router         OperatorType = "router"
	TraceParser    OperatorType = "trace_parser"
	Noop           OperatorType = "noop"
	JsonParser     OperatorType = "json_parser"
	Container      OperatorType = "container"
)

type TraceAttribute struct {
	TraceID    OperatorAttribute `yaml:"trace_id,omitempty"`
	SpanID     OperatorAttribute `yaml:"span_id,omitempty"`
	TraceFlags OperatorAttribute `yaml:"trace_flags,omitempty"`
}

type Route struct {
	Expression string `yaml:"expr,omitempty"`
	Output     string `yaml:"output,omitempty"`
}

type OperatorAttribute struct {
	ParseFrom string `yaml:"parse_from,omitempty"`
}

type Processors struct {
	config.BaseProcessors `yaml:",inline"`

	SetInstrumentationScopeRuntime *log.TransformProcessor            `yaml:"transform/set-instrumentation-scope-runtime,omitempty"`
	K8sAttributes                  *config.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes        *config.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	ResolveServiceName             *config.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes             *config.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`
}

type Exporters map[string]Exporter

type Exporter struct {
	OTLP *config.OTLPExporter `yaml:",inline,omitempty"`
}

type Extensions struct {
	config.Extensions `yaml:",inline"`

	FileStorage *FileStorage `yaml:"file_storage,omitempty"`
}

type FileStorage struct {
	Directory string `yaml:"directory,omitempty"`
}
