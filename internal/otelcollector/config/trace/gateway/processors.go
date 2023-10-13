package gateway

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
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
				LimitPercentage:      60,
				SpikeLimitPercentage: 40,
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

const (
	methodIsGet                        = "attributes[\"http.method\"] == \"GET\""
	methodIsPost                       = "attributes[\"http.method\"] == \"POST\""
	componentIsProxy                   = "attributes[\"component\"] == \"proxy\""
	operationIsIngress                 = "(attributes[\"OperationName\"] == \"Ingress\" or IsMatch(name, \"ingress.*\") == true)"
	operationIsEgress                  = "(attributes[\"OperationName\"] == \"Egress\" or IsMatch(name, \"egress.*\") == true)"
	namespacesIsKymaSystem             = "resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\""
	namespacesIsIstioSystem            = "resource.attributes[\"k8s.namespace.name\"] == \"istio-system\""
	istioNameIsGrafana                 = "attributes[\"istio.canonical_service\"] == \"grafana\""
	istioNameIsAuthProxy               = "attributes[\"istio.canonical_service\"] == \"monitoring-auth-proxy-grafana\""
	istioNameIsFluentBit               = "attributes[\"istio.canonical_service\"] == \"telemetry-fluent-bit\""
	istioNameIsTraceGateway            = "attributes[\"istio.canonical_service\"] == \"telemetry-trace-collector\""
	istioNameIsMetricAgent             = "attributes[\"istio.canonical_service\"] == \"telemetry-metric-agent\""
	istioNameIsMetricGateway           = "attributes[\"istio.canonical_service\"] == \"telemetry-metric-gateway\""
	istioNameIsIstioGateway            = "attributes[\"istio.canonical_service\"] == \"istio-ingressgateway\""
	userAgentIsVmScrapeAgent           = "attributes[\"user_agent\"] == \"vm_promscrape\""
	userAgentIsPrometheus              = "IsMatch(attributes[\"user_agent\"], \"Prometheus\\\\/.*\") == true"
	userAgentIsKymaOtelCol             = "IsMatch(attributes[\"user_agent\"], \"kyma-otelcol\\\\/.*\") == true"
	urlIsIstioHealthz                  = "IsMatch(attributes[\"http.url\"], \"https:\\\\/\\\\/healthz\\\\..+\\\\/healthz\\\\/ready\") == true"
	urlIsTelemetryTraceService         = "IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(4317|4318).*\") == true"
	urlIsTelemetryTraceInternalService = "IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-trace-collector-internal\\\\.kyma-system(\\\\..*)?:(55678).*\") == true"
	urlIsTelemetryMetricService        = "IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-otlp-metrics\\\\.kyma-system(\\\\..*)?:(4317|4318).*\") == true"
	and                                = " and "

	toFromKymaGrafana            = componentIsProxy + and + namespacesIsKymaSystem + and + istioNameIsGrafana
	toFromKymaAuthProxy          = componentIsProxy + and + namespacesIsKymaSystem + and + istioNameIsAuthProxy
	toFromTelemetryFluentBit     = componentIsProxy + and + namespacesIsKymaSystem + and + istioNameIsFluentBit
	toFromTelemetryTraceGateway  = componentIsProxy + and + namespacesIsKymaSystem + and + istioNameIsTraceGateway
	toFromTelemetryMetricGateway = componentIsProxy + and + namespacesIsKymaSystem + and + istioNameIsMetricGateway
	toFromTelemetryMetricAgent   = componentIsProxy + and + namespacesIsKymaSystem + and + istioNameIsMetricAgent

	toIstioGatewayWitHealthz = componentIsProxy + and + namespacesIsIstioSystem + and + methodIsGet + and + operationIsEgress + and + istioNameIsIstioGateway + and + urlIsIstioHealthz

	toTelemetryTraceService         = componentIsProxy + and + methodIsPost + and + operationIsEgress + and + urlIsTelemetryTraceService
	toTelemetryTraceInternalService = componentIsProxy + and + methodIsPost + and + operationIsEgress + and + urlIsTelemetryTraceInternalService
	toTelemetryMetricService        = componentIsProxy + and + methodIsPost + and + operationIsEgress + and + urlIsTelemetryMetricService

	//TODO: should be system namespaces after solving https://github.com/kyma-project/telemetry-manager/issues/380
	fromVmScrapeAgent        = componentIsProxy + and + methodIsGet + and + operationIsIngress + and + userAgentIsVmScrapeAgent
	fromPrometheusWithinKyma = componentIsProxy + and + methodIsGet + and + operationIsIngress + and + namespacesIsKymaSystem + and + userAgentIsPrometheus
	fromTelemetryMetricAgent = componentIsProxy + and + methodIsGet + and + operationIsIngress + and + userAgentIsKymaOtelCol
)

func makeSpanFilterConfig() []string {
	return []string{
		toFromKymaGrafana,
		toFromKymaAuthProxy,
		toFromTelemetryFluentBit,
		toFromTelemetryTraceGateway,
		toFromTelemetryMetricGateway,
		toFromTelemetryMetricAgent,
		toIstioGatewayWitHealthz,
		toTelemetryTraceService,
		toTelemetryTraceInternalService,
		toTelemetryMetricService,
		fromVmScrapeAgent,
		fromPrometheusWithinKyma,
		fromTelemetryMetricAgent,
	}
}
