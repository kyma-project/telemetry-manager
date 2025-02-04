package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
)

func makeProcessorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		K8sAttributes:           gatewayprocs.K8sAttributesProcessorConfig(),
		InsertClusterAttributes: gatewayprocs.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.CloudProvider),
		DropNoisySpans:          makeDropNoisySpansConfig(),
		ResolveServiceName:      makeResolveServiceNameConfig(),
		DropKymaAttributes:      gatewayprocs.DropKymaAttributesProcessorConfig(),
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

func makeResolveServiceNameConfig() *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:       "ignore",
		TraceStatements: gatewayprocs.ResolveServiceNameStatements(),
	}
}
