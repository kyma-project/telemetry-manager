package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/gatewayprocs"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func makeProcessorsConfig() Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch:         makeBatchProcessorConfig(),
			MemoryLimiter: makeMemoryLimiterConfig(),
			K8sAttributes: gatewayprocs.MakeK8sAttributesProcessorConfig(),
			Resource:      makeResourceProcessorConfig(),
		},
		CumulativeToDelta:  &CumulativeToDeltaProcessor{},
		ResolveServiceName: makeResolveServiceNameConfig(),
	}
}

func makeBatchProcessorConfig() *config.BatchProcessor {
	return &config.BatchProcessor{
		SendBatchSize:    1024,
		Timeout:          "10s",
		SendBatchMaxSize: 1024,
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "0.1s",
		LimitPercentage:      75,
		SpikeLimitPercentage: 10,
	}
}

func makeResourceProcessorConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    "k8s.cluster.name",
				Value:  "${KUBERNETES_SERVICE_HOST}",
			},
		},
	}
}

func makeDropIfInputSourceRuntimeConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetric{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceRuntime),
			},
		},
	}
}

func makeDropIfInputSourcePrometheusConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetric{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourcePrometheus),
			},
		},
	}
}

func makeDropIfInputSourceIstioConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetric{
			DataPoint: []string{
				fmt.Sprintf("resource.attributes[\"%s\"] == \"%s\"", metric.InputSourceAttribute, metric.InputSourceIstio),
			},
		},
	}
}

func makeResolveServiceNameConfig() *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:        "ignore",
		MetricStatements: gatewayprocs.MakeResolveServiceNameStatements(),
	}
}
