package common

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

const (
	defaultTransformProcessorErrorMode = "ignore"
)

type BaseProcessors struct {
	Batch         *BatchProcessor `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiter  `yaml:"memory_limiter,omitempty"`
}

type BatchProcessor struct {
	SendBatchSize    int    `yaml:"send_batch_size"`
	Timeout          string `yaml:"timeout"`
	SendBatchMaxSize int    `yaml:"send_batch_max_size"`
}

type MemoryLimiter struct {
	CheckInterval        string `yaml:"check_interval"`
	LimitPercentage      int    `yaml:"limit_percentage"`
	SpikeLimitPercentage int    `yaml:"spike_limit_percentage"`
}

type K8sAttributesProcessor struct {
	AuthType       string             `yaml:"auth_type"`
	Passthrough    bool               `yaml:"passthrough"`
	Extract        ExtractK8sMetadata `yaml:"extract"`
	PodAssociation []PodAssociations  `yaml:"pod_association"`
}

type ExtractK8sMetadata struct {
	Metadata []string       `yaml:"metadata"`
	Labels   []ExtractLabel `yaml:"labels"`
}

type ExtractLabel struct {
	From     string `yaml:"from"`
	Key      string `yaml:"key,omitempty"`
	TagName  string `yaml:"tag_name"`
	KeyRegex string `yaml:"key_regex,omitempty"`
}

type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

type ResourceProcessor struct {
	Attributes []AttributeAction `yaml:"attributes"`
}

type AttributeAction struct {
	Action       string `yaml:"action,omitempty"`
	Key          string `yaml:"key,omitempty"`
	Value        string `yaml:"value,omitempty"`
	RegexPattern string `yaml:"pattern,omitempty"`
}

type TransformProcessor struct {
	ErrorMode        string                         `yaml:"error_mode"`
	LogStatements    []TransformProcessorStatements `yaml:"log_statements,omitempty"`
	MetricStatements []TransformProcessorStatements `yaml:"metric_statements,omitempty"`
	TraceStatements  []TransformProcessorStatements `yaml:"trace_statements,omitempty"`
}

type TransformProcessorStatements struct {
	Statements []string `yaml:"statements"`
	Conditions []string `yaml:"conditions,omitempty"`
}

// LogTransformProcessor creates a TransformProcessor for logs with error_mode set to "ignore".
func LogTransformProcessor(statements []TransformProcessorStatements) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:     defaultTransformProcessorErrorMode,
		LogStatements: statements,
	}
}

// MetricTransformProcessor creates a TransformProcessor for metrics with the default error mode.
func MetricTransformProcessor(statements []TransformProcessorStatements) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:        defaultTransformProcessorErrorMode,
		MetricStatements: statements,
	}
}

// TraceTransformProcessor creates a TransformProcessor for traces with the default error mode.
func TraceTransformProcessor(statements []TransformProcessorStatements) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:       defaultTransformProcessorErrorMode,
		TraceStatements: statements,
	}
}

func TransformSpecsToProcessorStatements(specs []telemetryv1alpha1.TransformSpec) []TransformProcessorStatements {
	result := make([]TransformProcessorStatements, 0, len(specs))
	for _, spec := range specs {
		result = append(result, TransformProcessorStatements{
			Statements: spec.Statements,
			Conditions: spec.Conditions,
		})
	}

	return result
}

type IstioNoiseFilterProcessor struct {
}
