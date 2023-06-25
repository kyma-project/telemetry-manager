package metricpipeline

import (
	"context"
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	configbuilder "github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder"
)

func makeOtelCollectorConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.MetricPipeline) (*config.Config, configbuilder.EnvVars, error) {
	allVars := make(configbuilder.EnvVars)
	exportersConfig := make(config.ExportersConfig)
	pipelineConfigs := make(map[string]config.PipelineConfig)

	for _, pipeline := range pipelines {
		if pipeline.DeletionTimestamp != nil {
			continue
		}

		output := pipeline.Spec.Output
		queueSize := 256 / len(pipelines)
		exporterConfig, envVars, err := configbuilder.MakeOTLPExportersConfig(ctx, c, output.Otlp, pipeline.Name, queueSize)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to make exporter config: %v", err)
		}

		var outputAliases []string
		for k, v := range exporterConfig {
			exportersConfig[k] = v
			outputAliases = append(outputAliases, k)
		}

		processorAliases := getProcessorAliases(&output)

		sort.Strings(outputAliases)
		sort.Strings(processorAliases)

		pipelineConfig := makePipelineConfig(outputAliases, processorAliases)
		pipelineName := fmt.Sprintf("metrics/%s", pipeline.Name)
		pipelineConfigs[pipelineName] = pipelineConfig

		for k, v := range envVars {
			allVars[k] = v
		}
	}

	receiverConfig := makeReceiversConfig()
	processorsConfig := makeProcessorsConfig()
	serviceConfig := configbuilder.MakeServiceConfig(pipelineConfigs)
	extensionConfig := configbuilder.MakeExtensionsConfig()

	return &config.Config{
		Exporters:  exportersConfig,
		Receivers:  receiverConfig,
		Processors: processorsConfig,
		Service:    serviceConfig,
		Extensions: extensionConfig,
	}, allVars, nil
}

func getProcessorAliases(output *v1alpha1.MetricPipelineOutput) []string {
	if output.ToDelta {
		return []string{"cumulativetodelta", "memory_limiter", "k8sattributes", "resource", "batch"}
	}
	return []string{"memory_limiter", "k8sattributes", "resource", "batch"}
}

func makeReceiversConfig() config.ReceiversConfig {
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
		CumulativeToDelta: &config.CumulativeToDeltaConfig{},
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

func makePipelineConfig(outputAliases []string, processorAliases []string) config.PipelineConfig {
	return config.PipelineConfig{
		Receivers:  []string{"otlp"},
		Processors: processorAliases,
		Exporters:  outputAliases,
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
