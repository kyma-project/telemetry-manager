package tracepipeline

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

func makeGatewayConfig(ctx context.Context, c client.Reader, pipelines []v1alpha1.TracePipeline) (*config.Config, configbuilder.EnvVars, error) {
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
		sort.Strings(outputAliases)
		pipelineConfig := makeGatewayPipelineConfig(outputAliases)
		pipelineName := fmt.Sprintf("traces/%s", pipeline.Name)
		pipelineConfigs[pipelineName] = pipelineConfig

		for k, v := range envVars {
			allVars[k] = v
		}
	}

	receiverConfig := makeGatewayReceiversConfig()
	processorsConfig := makeGatewayProcessorsConfig()
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

func makeGatewayReceiversConfig() config.ReceiversConfig {
	return config.ReceiversConfig{
		OpenCensus: &config.EndpointConfig{
			Endpoint: "${MY_POD_IP}:55678",
		},
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
		Filter: &config.FilterProcessorConfig{
			Traces: config.TraceConfig{
				Span: makeGatewaySpanFilterConfig(),
			},
		},
	}
}

func makeGatewaySpanFilterConfig() []string {
	return []string{
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (resource.attributes[\"service.name\"] == \"grafana.kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"grafana.kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"loki.kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (IsMatch(attributes[\"http.url\"], \".+/metrics\") == true) and (resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (IsMatch(attributes[\"http.url\"], \".+/healthz(/.*)?\") == true) and (resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (attributes[\"user_agent\"] == \"vm_promscrape\")",
		"(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(4318|4317).*\") == true)",
		"(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-trace-collector-internal\\\\.kyma-system(\\\\..*)?:(55678).*\") == true)",
		"(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"loki.kyma-system\")",
		"(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (resource.attributes[\"service.name\"] == \"telemetry-fluent-bit.kyma-system\")",
	}
}

func makeGatewayPipelineConfig(outputAliases []string) config.PipelineConfig {
	return config.PipelineConfig{
		Receivers:  []string{"opencensus", "otlp"},
		Processors: []string{"memory_limiter", "k8sattributes", "filter", "resource", "batch"},
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
