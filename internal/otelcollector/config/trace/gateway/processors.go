package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
)

func makeProcessorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		K8sAttributes: processors.K8sAttributesProcessorConfig(processors.Enrichments{
			Enabled: false,
		}),
		InsertClusterAttributes: processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.CloudProvider),
		DropNoisySpans:          makeDropNoisySpansConfig(),
		ResolveServiceName:      makeResolveServiceNameConfig(),
		DropKymaAttributes:      processors.DropKymaAttributesProcessorConfig(),
	}
}

//nolint:mnd // hardcoded values
func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    512,
		Timeout:          "10s",
		SendBatchMaxSize: 512,
	}
}

//nolint:mnd // hardcoded values
func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}

func makeResolveServiceNameConfig() *config.ServiceEnrichmentProcessor {
	return &config.ServiceEnrichmentProcessor{
		CustomLabels: []string{
			"kyma.kubernetes_io_app_name",
			"kyma.app_name"},
	}
}
