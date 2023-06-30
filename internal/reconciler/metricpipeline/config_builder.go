package metricpipeline

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/otlpoutput"
)

func makeGatewayConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.MetricPipeline) (*config.Config, otlpoutput.EnvVars, error) {
	allVars := make(otlpoutput.EnvVars)
	exportersConfig := make(config.ExportersConfig)
	pipelinesConfig := make(config.PipelinesConfig)

	for _, pipeline := range pipelines {
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		output := pipeline.Spec.Output
		queueSize := 256 / len(pipelines)
		exporterConfig, envVars, err := otlpoutput.MakeExportersConfig(ctx, c, output.Otlp, pipeline.Name, queueSize)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to make exporter config: %v", err)
		}

		var outputAliases []string
		for k, v := range exporterConfig {
			exportersConfig[k] = v
			outputAliases = append(outputAliases, k)
		}
		sort.Strings(outputAliases)
		pipelineConfig := makeGatewayPipelineConfig(outputAliases)
		pipelineName := fmt.Sprintf("metrics/%s", pipeline.Name)
		pipelinesConfig[pipelineName] = pipelineConfig

		for k, v := range envVars {
			allVars[k] = v
		}
	}

	receiverConfig := makeGatewayReceiversConfig()
	processorsConfig := makeGatewayProcessorsConfig()
	serviceConfig := makeGatewayServiceConfig(pipelinesConfig)
	extensionConfig := makeGatewayExtensionsConfig()

	return &config.Config{
		Exporters:  exportersConfig,
		Receivers:  receiverConfig,
		Processors: processorsConfig,
		Service:    serviceConfig,
		Extensions: extensionConfig,
	}, allVars, nil
}

func makeGatewayReceiversConfig() config.ReceiversConfig {
	return config.ReceiversConfig{
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

func makeGatewayProcessorsConfig() config.ProcessorsConfig {
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

func makeGatewayPipelineConfig(outputAliases []string) config.PipelineConfig {
	return config.PipelineConfig{
		Receivers:  []string{"otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "resource", "batch"},
		Exporters:  outputAliases,
	}
}

func makeGatewayExtensionsConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: "${MY_POD_IP}:13133",
		},
		Pprof: config.EndpointConfig{
			Endpoint: "127.0.0.1:1777",
		},
	}
}

func makeGatewayServiceConfig(pipelines config.PipelinesConfig) config.ServiceConfig {
	return config.ServiceConfig{
		Pipelines: pipelines,
		Telemetry: config.TelemetryConfig{
			Metrics: config.MetricsConfig{
				Address: "${MY_POD_IP}:8888",
			},
			Logs: config.LoggingConfig{
				Level: "info",
			},
		},
		Extensions: []string{"health_check", "pprof"},
	}
}

func makeNetworkPolicyPorts() []intstr.IntOrString {
	return []intstr.IntOrString{
		intstr.FromInt(13133),
		intstr.FromInt(4317),
		intstr.FromInt(4318),
		intstr.FromInt(55678),
		intstr.FromInt(8888),
	}
}

func makeAgentConfig(gatewayServiceName types.NamespacedName, pipelines []v1alpha1.MetricPipeline) *config.Config {
	return &config.Config{
		Receivers:  makeAgentReceiversConfig(pipelines),
		Exporters:  makeAgentExportersConfig(gatewayServiceName),
		Extensions: makeGatewayExtensionsConfig(),
		Service:    makeAgentServiceConfig(),
	}
}

func makeAgentReceiversConfig(pipelines []v1alpha1.MetricPipeline) config.ReceiversConfig {
	enableRuntimeMetrics := false
	for i, _ := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Runtime.Enabled {
			enableRuntimeMetrics = true
		}
	}

	receiversConfig := config.ReceiversConfig{}
	if enableRuntimeMetrics {
		const collectionInterval = "30s"
		receiversConfig.KubeletStats = &config.KubeletStatsReceiverConfig{
			CollectionInterval: collectionInterval,
			AuthType:           "serviceAccount",
			Endpoint:           "https://${env:MY_NODE_NAME}:10250",
			InsecureSkipVerify: true,
		}
	}

	return receiversConfig
}

func makeAgentExportersConfig(gatewayServiceName types.NamespacedName) config.ExportersConfig {
	exportersConfig := make(config.ExportersConfig)
	exportersConfig["otlp"] = config.ExporterConfig{
		OTLPExporterConfig: &config.OTLPExporterConfig{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:4317", gatewayServiceName.Name, gatewayServiceName.Namespace),
			TLS: config.TLSConfig{
				Insecure: true,
			},
			SendingQueue: config.SendingQueueConfig{
				Enabled:   true,
				QueueSize: 512,
			},
			RetryOnFailure: config.RetryOnFailureConfig{
				Enabled:         true,
				InitialInterval: "5s",
				MaxInterval:     "30s",
				MaxElapsedTime:  "300s",
			},
		},
	}
	return exportersConfig
}

func makeAgentServiceConfig() config.ServiceConfig {
	pipelinesConfig := make(config.PipelinesConfig)
	pipelinesConfig["metrics"] = config.PipelineConfig{
		Receivers: []string{"kubeletstats"},
		Exporters: []string{"otlp"},
	}
	return config.ServiceConfig{
		Pipelines: pipelinesConfig,
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
