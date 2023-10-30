package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
)

func makeProcessorsConfig() Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		K8sAttributes:      gatewayprocs.K8sAttributesProcessorConfig(),
		InsertClusterName:  gatewayprocs.InsertClusterNameProcessorConfig(),
		DropNoisySpans:     makeDropNoisySpansConfig(),
		ResolveServiceName: makeResolveServiceNameConfig(),
		DropKymaAttributes: gatewayprocs.DropKymaAttributesProcessorConfig(),
	}
}

func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    512,
		Timeout:          "10s",
		SendBatchMaxSize: 512,
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "1s",
		LimitPercentage:      60,
		SpikeLimitPercentage: 40,
	}
}

func makeResolveServiceNameConfig() *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:       "ignore",
		TraceStatements: gatewayprocs.ResolveServiceNameStatements(),
	}
}
