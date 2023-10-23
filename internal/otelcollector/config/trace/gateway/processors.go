package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
)

func makeProcessorsConfig() Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch: &config.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			},
			MemoryLimiter: &config.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      60,
				SpikeLimitPercentage: 40,
			},
			K8sAttributes: gatewayprocs.MakeK8sAttributesProcessorConfig(),
			Resource: &config.ResourceProcessor{
				Attributes: []config.AttributeAction{
					{
						Action: "insert",
						Key:    "k8s.cluster.name",
						Value:  "${KUBERNETES_SERVICE_HOST}",
					},
				},
			},
		},
		SpanFilter: FilterProcessor{
			Traces: Traces{
				Span: makeSpanFilterConfig(),
			},
		},
		ResolveServiceName: makeResolveServiceNameConfig(),
	}
}

func makeResolveServiceNameConfig() *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:       "ignore",
		TraceStatements: gatewayprocs.MakeResolveServiceNameStatements(),
	}
}
