package metricpipeline

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/collector"
)

func makeProcessorsConfig() collector.ProcessorsConfig {
	k8sAttributes := []string{
		"k8s.pod.name",
		"k8s.node.name",
		"k8s.namespace.name",
		"k8s.deployment.name",
		"k8s.statefulset.name",
		"k8s.daemonset.name",
		"k8s.cronjob.name",
		"k8s.job.name",
	}

	podAssociations := []collector.PodAssociations{
		{
			Sources: []collector.PodAssociation{
				{
					From: "resource_attribute",
					Name: "k8s.pod.ip",
				},
			},
		},
		{
			Sources: []collector.PodAssociation{
				{
					From: "resource_attribute",
					Name: "k8s.pod.uid",
				},
			},
		},
		{
			Sources: []collector.PodAssociation{
				{
					From: "connection",
				},
			},
		},
	}
	return collector.ProcessorsConfig{
		Batch: &collector.BatchProcessorConfig{
			SendBatchSize:    512,
			Timeout:          "10s",
			SendBatchMaxSize: 512,
		},
		MemoryLimiter: &collector.MemoryLimiterConfig{
			CheckInterval:        "1s",
			LimitPercentage:      75,
			SpikeLimitPercentage: 10,
		},
		K8sAttributes: &collector.K8sAttributesProcessorConfig{
			AuthType:    "serviceAccount",
			Passthrough: false,
			Extract: collector.ExtractK8sMetadataConfig{
				Metadata: k8sAttributes,
			},
			PodAssociation: podAssociations,
		},
		Resource: &collector.ResourceProcessorConfig{
			Attributes: []collector.AttributeAction{
				{
					Action: "insert",
					Key:    "k8s.cluster.name",
					Value:  "${KUBERNETES_SERVICE_HOST}",
				},
			},
		},
	}
}

func makeReceiverConfig() collector.ReceiverConfig {
	return collector.ReceiverConfig{
		OTLP: &collector.OTLPReceiverConfig{
			Protocols: collector.ReceiverProtocols{
				HTTP: collector.EndpointConfig{
					Endpoint: "${MY_POD_IP}:4318",
				},
				GRPC: collector.EndpointConfig{
					Endpoint: "${MY_POD_IP}:4317",
				},
			},
		},
	}
}

func makeServiceConfig(outputType string) collector.OTLPServiceConfig {
	return collector.OTLPServiceConfig{
		Pipelines: collector.PipelinesConfig{
			Metrics: &collector.PipelineConfig{
				Receivers:  []string{"otlp"},
				Processors: []string{"memory_limiter", "k8sattributes", "resource", "batch"},
				Exporters:  []string{outputType, "logging"},
			},
		},
		Telemetry: collector.TelemetryConfig{
			Metrics: collector.MetricsConfig{
				Address: "${MY_POD_IP}:8888",
			},
			Logs: collector.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makeOtelCollectorConfig(output v1alpha1.MetricPipelineOutput, isInsecureOutput bool) collector.OTELCollectorConfig {
	exporterConfig := collector.MakeExporterConfig(output.Otlp, isInsecureOutput)
	outputType := collector.GetOutputType(output.Otlp)
	processorsConfig := makeProcessorsConfig()
	receiverConfig := makeReceiverConfig()
	serviceConfig := makeServiceConfig(outputType)
	extensionConfig := collector.MakeExtensionConfig()

	return collector.OTELCollectorConfig{
		Exporters:  exporterConfig,
		Processors: processorsConfig,
		Receivers:  receiverConfig,
		Service:    serviceConfig,
		Extensions: extensionConfig,
	}
}
