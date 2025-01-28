package log

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

type TransformProcessor struct {
	ErrorMode     string                                `yaml:"error_mode"`
	LogStatements []config.TransformProcessorStatements `yaml:"log_statements"`
}
