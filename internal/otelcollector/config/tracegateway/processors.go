package tracegateway

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: common.BaseProcessors{
			Batch:         batchProcessorConfig(),
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		K8sAttributes:           common.K8sAttributesProcessorConfig(opts.Enrichments),
		IstioNoiseFilter:        &common.IstioNoiseFilterProcessor{},
		InsertClusterAttributes: common.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:      common.ResolveServiceNameConfig(),
		DropKymaAttributes:      common.DropKymaAttributesProcessorConfig(),
		Dynamic:                 make(map[string]any),
	}
}

//nolint:mnd // hardcoded values
func batchProcessorConfig() *common.BatchProcessor {
	return &common.BatchProcessor{
		SendBatchSize:    512,
		Timeout:          "10s",
		SendBatchMaxSize: 512,
	}
}

//nolint:mnd // hardcoded values
func memoryLimiterProcessorConfig() *common.MemoryLimiter {
	return &common.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 15,
	}
}
