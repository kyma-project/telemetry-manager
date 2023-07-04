package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func MakeConfig(gatewayServiceName types.NamespacedName, pipelines []v1alpha1.MetricPipeline) *config.Config {
	return &config.Config{
		Receivers:  makeReceiversConfig(pipelines),
		Exporters:  makeExportersConfig(gatewayServiceName),
		Extensions: makeExtensionsConfig(),
		Service:    makeServiceConfig(),
	}
}

func makeReceiversConfig(pipelines []v1alpha1.MetricPipeline) config.ReceiversConfig {
	enableRuntimeMetrics := false
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Runtime.Enabled {
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
			MetricGroups:       []config.MetricGroupType{config.MetricGroupTypeContainer, config.MetricGroupTypePod},
		}
	}

	return receiversConfig
}

func makeExportersConfig(gatewayServiceName types.NamespacedName) config.ExportersConfig {
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

func makeExtensionsConfig() config.ExtensionsConfig {
	return config.ExtensionsConfig{
		HealthCheck: config.EndpointConfig{
			Endpoint: "${MY_POD_IP}:13133",
		},
	}
}

func makeServiceConfig() config.ServiceConfig {
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
