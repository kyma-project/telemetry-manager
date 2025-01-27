package log

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

// TODO: abstract it out to common config as metrics has the same setup
type TransformProcessor struct {
	ErrorMode     string                                `yaml:"error_mode"`
	LogStatements []config.TransformProcessorStatements `yaml:"log_statements"`
}
