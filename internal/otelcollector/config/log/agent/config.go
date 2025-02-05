package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
)

type Config struct {
	Service    config.Service `yaml:"service"`
	Extensions Extensions     `yaml:"extensions"`

	Receivers  Receivers  `yaml:"receivers"`
	Processors Processors `yaml:"processors"`
	Exporters  Exporters  `yaml:"exporters"`
}

type Receivers struct {
	FileLog *FileLog `yaml:"filelog"`
}

type FileLog struct {
	Exclude         []string              `yaml:"exclude,omitempty"`
	Include         []string              `yaml:"include,omitempty"`
	IncludeFileName bool                  `yaml:"include_file_name,omitempty"`
	IncludeFilePath bool                  `yaml:"include_file_path,omitempty"`
	StartAt         string                `yaml:"start_at,omitempty"`
	Storage         string                `yaml:"storage,omitempty"`
	RetryOnFailure  config.RetryOnFailure `yaml:"retry_on_failure,omitempty"`
	Operators       []Operator            `yaml:"operators,omitempty"`
}

type Operator struct {
	ID                      string `yaml:"id,omitempty"`
	Type                    string `yaml:"type,omitempty"`
	AddMetadataFromFilePath *bool  `yaml:"add_metadata_from_file_path,omitempty"`
	Format                  string `yaml:"format,omitempty"`
	From                    string `yaml:"from,omitempty"`
	To                      string `yaml:"to,omitempty"`
	IfExpr                  string `yaml:"if,omitempty"`
	ParseFrom               string `yaml:"parse_from,omitempty"`
	ParseTo                 string `yaml:"parse_to,omitempty"`
}

type Processors struct {
	config.BaseProcessors          `yaml:",inline"`
	SetInstrumentationScopeRuntime *log.TransformProcessor `yaml:"transform/set-instrumentation-scope-runtime,omitempty"`
}

type Exporters struct {
	OTLP *config.OTLPExporter `yaml:"otlp"`
}

type Extensions struct {
	config.Extensions `yaml:",inline"`
	FileStorage       *FileStorage `yaml:"file_storage,omitempty"`
}

type FileStorage struct {
	Directory string `yaml:"directory,omitempty"`
}
