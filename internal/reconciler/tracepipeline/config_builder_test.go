package tracepipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

var (
	tracePipeline = v1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: v1alpha1.TracePipelineSpec{
			Output: v1alpha1.TracePipelineOutput{
				Otlp: &v1alpha1.OtlpOutput{
					Endpoint: v1alpha1.ValueType{
						Value: "localhost",
					},
				},
			},
		},
	}

	tracePipelineInsecure = v1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-insecure",
		},
		Spec: v1alpha1.TracePipelineSpec{
			Output: v1alpha1.TracePipelineOutput{
				Otlp: &v1alpha1.OtlpOutput{
					Endpoint: v1alpha1.ValueType{
						Value: "http://localhost",
					},
				},
			},
		},
	}

	tracePipelineHTTP = v1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-http",
		},
		Spec: v1alpha1.TracePipelineSpec{
			Output: v1alpha1.TracePipelineOutput{
				Otlp: &v1alpha1.OtlpOutput{
					Protocol: "http",
					Endpoint: v1alpha1.ValueType{
						Value: "localhost",
					},
				},
			},
		},
	}

	tracePipelineWithBasicAuth = v1alpha1.TracePipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-basic-auth",
		},
		Spec: v1alpha1.TracePipelineSpec{
			Output: v1alpha1.TracePipelineOutput{
				Otlp: &v1alpha1.OtlpOutput{
					Endpoint: v1alpha1.ValueType{
						Value: "localhost",
					},
					Authentication: &v1alpha1.AuthenticationOptions{
						Basic: &v1alpha1.BasicAuthOptions{
							User: v1alpha1.ValueType{
								Value: "user",
							},
							Password: v1alpha1.ValueType{
								Value: "password",
							},
						},
					},
				},
			},
		},
	}
)

func TestMakeCollectorConfigEndpoint(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipeline})
	require.NoError(t, err)
	expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")
	require.Contains(t, collectorConfig.Exporters, "otlp/test")
	otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
	require.Equal(t, expectedEndpoint, otlpExporterConfig.Endpoint)
}

func TestMakeCollectorConfigSecure(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipeline})
	require.NoError(t, err)
	require.Contains(t, collectorConfig.Exporters, "otlp/test")
	otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
	require.False(t, otlpExporterConfig.TLS.Insecure)
}

func TestMakeCollectorConfigSecureHttp(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipelineHTTP})
	require.NoError(t, err)
	require.Contains(t, collectorConfig.Exporters, "otlphttp/test-http")
	otlpExporterConfig := collectorConfig.Exporters["otlphttp/test-http"]
	require.False(t, otlpExporterConfig.TLS.Insecure)
}

func TestMakeCollectorConfigInsecure(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipelineInsecure})
	require.NoError(t, err)
	require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")
	otlpExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
	require.True(t, otlpExporterConfig.TLS.Insecure)
}

func TestMakeCollectorConfigMultiPipeline(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipeline, tracePipelineInsecure})
	require.NoError(t, err)

	require.Contains(t, collectorConfig.Exporters, "otlp/test")
	require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")

	require.Contains(t, collectorConfig.Service.Pipelines, "traces/test")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Exporters, "otlp/test")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Exporters, "logging/test")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Receivers, "otlp")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Receivers, "opencensus")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[0], "memory_limiter")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[1], "k8sattributes")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[2], "filter")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[3], "resource")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[4], "batch")

	require.Contains(t, collectorConfig.Service.Pipelines, "traces/test-insecure")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Exporters, "otlp/test-insecure")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Exporters, "logging/test-insecure")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Receivers, "otlp")
	require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Receivers, "opencensus")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Processors[0], "memory_limiter")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Processors[1], "k8sattributes")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Processors[2], "filter")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Processors[3], "resource")
	require.Equal(t, collectorConfig.Service.Pipelines["traces/test-insecure"].Processors[4], "batch")
}

func TestMakeCollectorConfigWithBasicAuth(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipelineWithBasicAuth})
	require.NoError(t, err)
	require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")
	otlpExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
	headers := otlpExporterConfig.Headers

	authHeader, existing := headers["Authorization"]
	require.True(t, existing)
	require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)
}

func TestMakeServiceConfig(t *testing.T) {
	pipelineConfig := map[string]config.PipelineConfig{
		"traces/test": makePipelineConfig([]string{"otlp/test", "logging/test"}),
	}
	serviceConfig := makeServiceConfig(pipelineConfig)

	require.Contains(t, serviceConfig.Pipelines, "traces/test")
	require.Contains(t, serviceConfig.Pipelines["traces/test"].Receivers, "otlp")
	require.Contains(t, serviceConfig.Pipelines["traces/test"].Receivers, "opencensus")

	require.Equal(t, serviceConfig.Pipelines["traces/test"].Processors[0], "memory_limiter")
	require.Equal(t, serviceConfig.Pipelines["traces/test"].Processors[1], "k8sattributes")
	require.Equal(t, serviceConfig.Pipelines["traces/test"].Processors[2], "filter")
	require.Equal(t, serviceConfig.Pipelines["traces/test"].Processors[3], "resource")
	require.Equal(t, serviceConfig.Pipelines["traces/test"].Processors[4], "batch")

	require.Contains(t, serviceConfig.Pipelines["traces/test"].Exporters, "otlp/test")
	require.Contains(t, serviceConfig.Pipelines["traces/test"].Exporters, "logging/test")

	require.Equal(t, "${MY_POD_IP}:8888", serviceConfig.Telemetry.Metrics.Address)
	require.Equal(t, "info", serviceConfig.Telemetry.Logs.Level)
	require.Contains(t, serviceConfig.Extensions, "health_check")
}

func TestResourceProcessors(t *testing.T) {
	processors := makeProcessorsConfig()

	require.Equal(t, 1, len(processors.Resource.Attributes))
	require.Equal(t, "insert", processors.Resource.Attributes[0].Action)
	require.Equal(t, "k8s.cluster.name", processors.Resource.Attributes[0].Key)
	require.Equal(t, "${KUBERNETES_SERVICE_HOST}", processors.Resource.Attributes[0].Value)

}

func TestMemoryLimiterProcessor(t *testing.T) {
	processors := makeProcessorsConfig()

	require.Equal(t, "1s", processors.MemoryLimiter.CheckInterval)
	require.Equal(t, 75, processors.MemoryLimiter.LimitPercentage)
	require.Equal(t, 10, processors.MemoryLimiter.SpikeLimitPercentage)
}

func TestBatchProcessor(t *testing.T) {
	processors := makeProcessorsConfig()

	require.Equal(t, 512, processors.Batch.SendBatchSize)
	require.Equal(t, 512, processors.Batch.SendBatchMaxSize)
	require.Equal(t, "10s", processors.Batch.Timeout)
}

func TestK8sAttributesProcessor(t *testing.T) {
	processors := makeProcessorsConfig()

	require.Equal(t, "serviceAccount", processors.K8sAttributes.AuthType)
	require.False(t, processors.K8sAttributes.Passthrough)

	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.pod.name")

	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.node.name")
	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.namespace.name")
	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.deployment.name")

	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.statefulset.name")
	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.daemonset.name")
	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.cronjob.name")
	require.Contains(t, processors.K8sAttributes.Extract.Metadata, "k8s.job.name")

	require.Equal(t, 3, len(processors.K8sAttributes.PodAssociation))
	require.Equal(t, "resource_attribute", processors.K8sAttributes.PodAssociation[0].Sources[0].From)
	require.Equal(t, "k8s.pod.ip", processors.K8sAttributes.PodAssociation[0].Sources[0].Name)

	require.Equal(t, "resource_attribute", processors.K8sAttributes.PodAssociation[1].Sources[0].From)
	require.Equal(t, "k8s.pod.uid", processors.K8sAttributes.PodAssociation[1].Sources[0].Name)

	require.Equal(t, "connection", processors.K8sAttributes.PodAssociation[2].Sources[0].From)
}

func TestFilterProcessor(t *testing.T) {
	processors := makeProcessorsConfig()
	require.Equal(t, len(processors.Filter.Traces.Span), 10, "Span filter list size is wrong")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (resource.attributes[\"service.name\"] == \"grafana.kyma-system\")", "Grafana span filter egress missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"grafana.kyma-system\")", "Grafana span filter ingress GET missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"loki.kyma-system\")", "Loki span filter ingress GET missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (IsMatch(attributes[\"http.url\"], \".+/metrics\") == true) and (resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\")", "/metrics endpoint span filter missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (IsMatch(attributes[\"http.url\"], \".+/healthz(/.*)?\") == true) and (resource.attributes[\"k8s.namespace.name\"] == \"kyma-system\")", "/healthz endpoint span filter missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"GET\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (attributes[\"user_agent\"] == \"vm_promscrape\")", "Victoria Metrics agent span filter missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-otlp-traces\\\\.kyma-system(\\\\..*)?:(4318|4317).*\") == true)", "Telemetry OTLP service span filter missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (IsMatch(attributes[\"http.url\"], \"http(s)?:\\\\/\\\\/telemetry-trace-collector-internal\\\\.kyma-system(\\\\..*)?:(55678).*\") == true)", "Telemetry Opencensus service span filter missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Ingress\") and (resource.attributes[\"service.name\"] == \"loki.kyma-system\")", "Loki service span filter missing")
	require.Contains(t, processors.Filter.Traces.Span, "(attributes[\"http.method\"] == \"POST\") and (attributes[\"component\"] == \"proxy\") and (attributes[\"OperationName\"] == \"Egress\") and (resource.attributes[\"service.name\"] == \"telemetry-fluent-bit.kyma-system\")", "Fluent-Bit service span filter missing")
}

func TestCollectorConfigMarshalling(t *testing.T) {
	expected := `receivers:
    opencensus:
        endpoint: ${MY_POD_IP}:55678
    otlp:
        protocols:
            http:
                endpoint: ${MY_POD_IP}:4318
            grpc:
                endpoint: ${MY_POD_IP}:4317
exporters:
    logging/test:
        verbosity: basic
    otlp/test:
        endpoint: ${OTLP_ENDPOINT_TEST}
        sending_queue:
            enabled: true
            queue_size: 512
        retry_on_failure:
            enabled: true
            initial_interval: 5s
            max_interval: 30s
            max_elapsed_time: 300s
processors:
    batch:
        send_batch_size: 512
        timeout: 10s
        send_batch_max_size: 512
    memory_limiter:
        check_interval: 1s
        limit_percentage: 75
        spike_limit_percentage: 10
    k8sattributes:
        auth_type: serviceAccount
        passthrough: false
        extract:
            metadata:
                - k8s.pod.name
                - k8s.node.name
                - k8s.namespace.name
                - k8s.deployment.name
                - k8s.statefulset.name
                - k8s.daemonset.name
                - k8s.cronjob.name
                - k8s.job.name
        pod_association:
            - sources:
                - from: resource_attribute
                  name: k8s.pod.ip
            - sources:
                - from: resource_attribute
                  name: k8s.pod.uid
            - sources:
                - from: connection
    resource:
        attributes:
            - action: insert
              key: k8s.cluster.name
              value: ${KUBERNETES_SERVICE_HOST}
    filter:
        traces:
            span:
                - (attributes["http.method"] == "GET") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Egress") and (resource.attributes["service.name"] == "grafana.kyma-system")
                - (attributes["http.method"] == "GET") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Ingress") and (resource.attributes["service.name"] == "grafana.kyma-system")
                - (attributes["http.method"] == "GET") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Ingress") and (resource.attributes["service.name"] == "loki.kyma-system")
                - (attributes["http.method"] == "GET") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Ingress") and (IsMatch(attributes["http.url"], ".+/metrics") == true) and (resource.attributes["k8s.namespace.name"] == "kyma-system")
                - (attributes["http.method"] == "GET") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Ingress") and (IsMatch(attributes["http.url"], ".+/healthz(/.*)?") == true) and (resource.attributes["k8s.namespace.name"] == "kyma-system")
                - (attributes["http.method"] == "GET") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Ingress") and (attributes["user_agent"] == "vm_promscrape")
                - (attributes["http.method"] == "POST") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Egress") and (IsMatch(attributes["http.url"], "http(s)?:\\/\\/telemetry-otlp-traces\\.kyma-system(\\..*)?:(4318|4317).*") == true)
                - (attributes["http.method"] == "POST") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Egress") and (IsMatch(attributes["http.url"], "http(s)?:\\/\\/telemetry-trace-collector-internal\\.kyma-system(\\..*)?:(55678).*") == true)
                - (attributes["http.method"] == "POST") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Ingress") and (resource.attributes["service.name"] == "loki.kyma-system")
                - (attributes["http.method"] == "POST") and (attributes["component"] == "proxy") and (attributes["OperationName"] == "Egress") and (resource.attributes["service.name"] == "telemetry-fluent-bit.kyma-system")
extensions:
    health_check:
        endpoint: ${MY_POD_IP}:13133
service:
    pipelines:
        traces/test:
            receivers:
                - opencensus
                - otlp
            processors:
                - memory_limiter
                - k8sattributes
                - filter
                - resource
                - batch
            exporters:
                - logging/test
                - otlp/test
    telemetry:
        metrics:
            address: ${MY_POD_IP}:8888
        logs:
            level: info
    extensions:
        - health_check
`

	fakeClient := fake.NewClientBuilder().Build()
	collectorConfig, _, err := makeOtelCollectorConfig(context.Background(), fakeClient, []v1alpha1.TracePipeline{tracePipeline})
	require.NoError(t, err)

	yamlBytes, err := yaml.Marshal(collectorConfig)
	require.NoError(t, err)
	require.Equal(t, expected, string(yamlBytes))
}
