package metricpipeline

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	configbuilder "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder"
)

func makeOtelCollectorConfig(ctx context.Context, c client.Reader, pipeline *v1alpha1.MetricPipeline) (*config.Config, configbuilder.EnvVars, error) {
	output := pipeline.Spec.Output
	exporterConfig, envVars, err := configbuilder.MakeOTLPExporterConfig(ctx, c, output.Otlp, pipeline.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make exporter config: %v", err)
	}

	outputAliases := configbuilder.GetExporterAliases(exporterConfig)
	receiverConfig := makeReceiverConfig()
	processorsConfig := makeProcessorsConfig()
	serviceConfig := makeServiceConfig(outputAliases)
	extensionConfig := configbuilder.MakeExtensionConfig()

	return &config.Config{
		Exporters:  exporterConfig,
		Receivers:  receiverConfig,
		Processors: processorsConfig,
		Service:    serviceConfig,
		Extensions: extensionConfig,
	}, envVars, nil
}

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
			SendBatchSize:    1024,
			Timeout:          "10s",
			SendBatchMaxSize: 1024,
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

func makeServiceConfig(outputAliases []string) config.OTLPServiceConfig {
	pipelines := map[string]config.PipelineConfig{
		"metrics": {
			Receivers:  []string{"otlp"},
			Processors: []string{"memory_limiter", "k8sattributes", "resource", "batch"},
			Exporters:  outputAliases,
		},
	}
	return config.OTLPServiceConfig{
		Pipelines: pipelines,
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
