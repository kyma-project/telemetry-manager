package metric

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

type TransformProcessor struct {
	ErrorMode        string                                `yaml:"error_mode"`
	MetricStatements []config.TransformProcessorStatements `yaml:"metric_statements"`
}

type ServiceEnrichmentProcessor struct {
	CustomLabels []string `yaml:"custom_labels"`
}

type LeaderElection struct {
	LeaseName      string `yaml:"lease_name"`
	LeaseNamespace string `yaml:"lease_namespace"`
}
