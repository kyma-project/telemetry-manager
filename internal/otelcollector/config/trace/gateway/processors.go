package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
)

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		K8sAttributes:           processors.K8sAttributesProcessorConfig(opts.Enrichments),
		IstioNoiseFilter:        &config.IstioNoiseFilterProcessor{},
		InsertClusterAttributes: processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:      processors.MakeResolveServiceNameConfig(),
		DropKymaAttributes:      processors.DropKymaAttributesProcessorConfig(),
		Transforms:              make(map[string]*config.TransformProcessor),
	}
}

//nolint:mnd // hardcoded values
func batchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    512,
		Timeout:          "10s",
		SendBatchMaxSize: 512,
	}
}

//nolint:mnd // hardcoded values
func memoryLimiterProcessorConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}
