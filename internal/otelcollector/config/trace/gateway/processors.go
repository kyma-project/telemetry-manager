package gateway

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func makeProcessorsConfig() Processors {
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
	return Processors{
		BaseProcessors: config.BaseProcessors{
			Batch: &config.BatchProcessor{
				SendBatchSize:    512,
				Timeout:          "10s",
				SendBatchMaxSize: 512,
			},
			MemoryLimiter: &config.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 10,
			},
			K8sAttributes: &config.K8sAttributesProcessor{
				AuthType:    "serviceAccount",
				Passthrough: false,
				Extract: config.ExtractK8sMetadata{
					Metadata: k8sAttributes,
				},
				PodAssociation: podAssociations,
			},
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
	}
}

func makeSpanFilterConfig() []string {
	return []string{
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (resource.attributes[\"service.name\"] == \"grafana.kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"grafana.kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (IsMatch(attributes[\"http.url\"], \".+/metrics\") == true) and (resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (IsMatch(attributes[\"http.url\"], \".+/healthz(/.*)?\") == true) and (resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\")",
		"(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (attributes[\"user_agent\"] == \"vm_promscrape\")",
		fmt.Sprintf("(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(%d|%d).*\") == true)", ports.OTLPHTTP, ports.OTLPGRPC),
		fmt.Sprintf("(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-trace-collector-internal\\\\.kyma-system(\\\\..*)?:(%d).*\") == true)", ports.OpenCensus),
		"(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (resource.attributes[\"service.name\"] == \"telemetry-fluent-bit.kyma-system\")",
	}
}
