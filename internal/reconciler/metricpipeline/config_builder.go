package metricpipeline

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder"
)

func makeReceiverConfig() config.ReceiverConfig {
	return config.ReceiverConfig{
		OTLP: &config.OTLPReceiverConfig{
			Protocols: config.ReceiverProtocols{
				HTTP: config.EndpointConfig{
					Endpoint: "${MY_POD_IP}:4318",
				},
				GRPC: config.EndpointConfig{
					Endpoint: "${MY_POD_IP}:4317",
				},
			},
		},
	}
}

func makeProcessorsConfig() config.ProcessorsConfig {
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

	podAssociations := []config.PodAssociations{
		{
			Sources: []config.PodAssociation{
				{
					From: "resource_attribute",
					Name: "k8s.pod.ip",
				},
			},
		},
		{
			Sources: []config.PodAssociation{
				{
					From: "resource_attribute",
					Name: "k8s.pod.uid",
				},
			},
		},
		{
			Sources: []config.PodAssociation{
				{
					From: "connection",
				},
			},
		},
	}
	return config.ProcessorsConfig{
		Batch: &config.BatchProcessorConfig{
			SendBatchSize:    512,
			Timeout:          "10s",
			SendBatchMaxSize: 512,
		},
		MemoryLimiter: &config.MemoryLimiterConfig{
			CheckInterval:        "1s",
			LimitPercentage:      75,
			SpikeLimitPercentage: 10,
		},
		K8sAttributes: &config.K8sAttributesProcessorConfig{
			AuthType:    "serviceAccount",
			Passthrough: false,
			Extract: config.ExtractK8sMetadataConfig{
				Metadata: k8sAttributes,
			},
			PodAssociation: podAssociations,
		},
		Resource: &config.ResourceProcessorConfig{
			Attributes: []config.AttributeAction{
				{
					Action: "insert",
					Key:    "k8s.cluster.name",
					Value:  "${KUBERNETES_SERVICE_HOST}",
				},
			},
		},
	}
}

func makeServiceConfig(outputType string) config.OTLPServiceConfig {
	return config.OTLPServiceConfig{
		Pipelines: config.PipelinesConfig{
			Metrics: &config.PipelineConfig{
				Receivers:  []string{"otlp"},
				Processors: []string{"memory_limiter", "k8sattributes", "resource", "batch"},
				Exporters:  []string{outputType, "logging"},
			},
		},
		Telemetry: config.TelemetryConfig{
			Metrics: config.MetricsConfig{
				Address: "${MY_POD_IP}:8888",
			},
			Logs: config.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check"},
	}
}

func makeOtelCollectorConfig(output v1alpha1.MetricPipelineOutput, isInsecureOutput bool) config.Config {
	exporterConfig := builder.MakeExporterConfig(output.Otlp, isInsecureOutput)
	outputType := builder.GetOutputType(output.Otlp)
	processorsConfig := makeProcessorsConfig()
	receiverConfig := makeReceiverConfig()
	serviceConfig := makeServiceConfig(outputType)
	extensionConfig := builder.MakeExtensionConfig()

	return config.Config{
		Exporters:  exporterConfig,
		Receivers:  receiverConfig,
		Processors: processorsConfig,
		Service:    serviceConfig,
		Extensions: extensionConfig,
	}
}
