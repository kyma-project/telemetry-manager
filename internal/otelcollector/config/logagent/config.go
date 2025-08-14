package logagent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

type Config struct {
	Service    common.Service `yaml:"service"`
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
	RetryOnFailure  common.RetryOnFailure `yaml:"retry_on_failure,omitempty"`
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
	common.BaseProcessors `yaml:",inline"`

	// OTel Collector components with static IDs
	SetInstrumentationScopeRuntime *common.TransformProcessor         `yaml:"transform/set-instrumentation-scope-runtime,omitempty"`
	K8sAttributes                  *common.K8sAttributesProcessor     `yaml:"k8sattributes,omitempty"`
	InsertClusterAttributes        *common.ResourceProcessor          `yaml:"resource/insert-cluster-attributes,omitempty"`
	ResolveServiceName             *common.ServiceEnrichmentProcessor `yaml:"service_enrichment,omitempty"`
	DropKymaAttributes             *common.ResourceProcessor          `yaml:"resource/drop-kyma-attributes,omitempty"`

	// OTel Collector components with dynamic IDs that are pipeline name based
	Dynamic map[string]any `yaml:",inline,omitempty"`
}

type Exporters map[string]Exporter
type Exporter struct {
	OTLP *common.OTLPExporter `yaml:",inline,omitempty"`
}

type Extensions struct {
	common.Extensions `yaml:",inline"`

	FileStorage *FileStorage `yaml:"file_storage,omitempty"`
}

type FileStorage struct {
	Directory string `yaml:"directory,omitempty"`
}
