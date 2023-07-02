package metricpipeline

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestMakeGatewayConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().withName("test").build()})
		require.NoError(t, err)
		expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")
		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, actualExporterConfig.Endpoint)
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().withName("test").build()})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")
		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, actualExporterConfig.TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			newPipelineBuilder().withName("test-insecure").withEndpoint("http://localhost").build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			newPipelineBuilder().withName("test-basic-auth").withBasicAuth("user", "password").build(),
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := actualExporterConfig.Headers

		authHeader, existing := headers["Authorization"]
		require.True(t, existing)
		require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)
	})

	t.Run("resource processors", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})
		require.NoError(t, err)

		require.Equal(t, 1, len(collectorConfig.Processors.Resource.Attributes))
		require.Equal(t, "insert", collectorConfig.Processors.Resource.Attributes[0].Action)
		require.Equal(t, "k8s.cluster.name", collectorConfig.Processors.Resource.Attributes[0].Key)
		require.Equal(t, "${KUBERNETES_SERVICE_HOST}", collectorConfig.Processors.Resource.Attributes[0].Value)
	})

	t.Run("memory limit processors", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})
		require.NoError(t, err)

		require.Equal(t, "1s", collectorConfig.Processors.MemoryLimiter.CheckInterval)
		require.Equal(t, 75, collectorConfig.Processors.MemoryLimiter.LimitPercentage)
		require.Equal(t, 10, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage)
	})

	t.Run("batch processors", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})
		require.NoError(t, err)

		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchSize)
		require.Equal(t, 1024, collectorConfig.Processors.Batch.SendBatchMaxSize)
		require.Equal(t, "10s", collectorConfig.Processors.Batch.Timeout)
	})

	t.Run("k8s attributes processors", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})
		require.NoError(t, err)

		require.Equal(t, "serviceAccount", collectorConfig.Processors.K8sAttributes.AuthType)
		require.False(t, collectorConfig.Processors.K8sAttributes.Passthrough)

		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.pod.name")

		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.node.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.namespace.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.deployment.name")

		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.statefulset.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.daemonset.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.cronjob.name")
		require.Contains(t, collectorConfig.Processors.K8sAttributes.Extract.Metadata, "k8s.job.name")

		require.Equal(t, 3, len(collectorConfig.Processors.K8sAttributes.PodAssociation))
		require.Equal(t, "resource_attribute", collectorConfig.Processors.K8sAttributes.PodAssociation[0].Sources[0].From)
		require.Equal(t, "k8s.pod.ip", collectorConfig.Processors.K8sAttributes.PodAssociation[0].Sources[0].Name)

		require.Equal(t, "resource_attribute", collectorConfig.Processors.K8sAttributes.PodAssociation[1].Sources[0].From)
		require.Equal(t, "k8s.pod.uid", collectorConfig.Processors.K8sAttributes.PodAssociation[1].Sources[0].Name)

		require.Equal(t, "connection", collectorConfig.Processors.K8sAttributes.PodAssociation[2].Sources[0].From)
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.NotEmpty(t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})
		require.NoError(t, err)

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().withName("test").build()})
		require.NoError(t, err)
		require.Equal(t, 256, collectorConfig.Exporters["otlp/test"].SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			newPipelineBuilder().withName("test-1").build(),
			newPipelineBuilder().withName("test-2").build(),
			newPipelineBuilder().withName("test-3").build()},
		)
		require.NoError(t, err)
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-1"].SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-2"].SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-3"].SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().withName("test").build()})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "logging/test")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test"].Processors[1], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test"].Processors[2], "resource")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test"].Processors[3], "batch")
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			newPipelineBuilder().withName("test-1").build(),
			newPipelineBuilder().withName("test-2").build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Exporters, "logging/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Receivers, "otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-1"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-1"].Processors[1], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-1"].Processors[2], "resource")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-1"].Processors[3], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Exporters, "logging/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Receivers, "otlp")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-2"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-2"].Processors[1], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-2"].Processors[2], "resource")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/test-2"].Processors[3], "batch")
	})

	t.Run("marshaling", func(t *testing.T) {
		expected := `receivers:
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
            queue_size: 256
        retry_on_failure:
            enabled: true
            initial_interval: 5s
            max_interval: 30s
            max_elapsed_time: 300s
processors:
    batch:
        send_batch_size: 1024
        timeout: 10s
        send_batch_max_size: 1024
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
extensions:
    health_check:
        endpoint: ${MY_POD_IP}:13133
    pprof:
        endpoint: 127.0.0.1:1777
service:
    pipelines:
        metrics/test:
            receivers:
                - otlp
            processors:
                - memory_limiter
                - k8sattributes
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
        - pprof
`

		collectorConfig, _, err := makeGatewayConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{newPipelineBuilder().withName("test").build()})
		require.NoError(t, err)

		yamlBytes, err := yaml.Marshal(collectorConfig)
		require.NoError(t, err)
		require.Equal(t, expected, string(yamlBytes))
	})
}

func TestMakeAgentConfig(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})

		require.Contains(t, collectorConfig.Exporters, "otlp")
		actualExporterConfig := collectorConfig.Exporters["otlp"]
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})

			require.Contains(t, collectorConfig.Exporters, "otlp")
			actualExporterConfig := collectorConfig.Exporters["otlp"]
			require.True(t, actualExporterConfig.TLS.Insecure)
		})
	})

	t.Run("no pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{
			newPipelineBuilder().build(),
			newPipelineBuilder().build(),
		})

		require.Empty(t, collectorConfig.Receivers.KubeletStats)
	})

	t.Run("some pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{
			newPipelineBuilder().withRuntimeInputOn(false).build(),
			newPipelineBuilder().withRuntimeInputOn(true).build(),
		})

		require.NotEmpty(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, true, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)
	})

	t.Run("all pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{
			newPipelineBuilder().withRuntimeInputOn(true).build(),
			newPipelineBuilder().withRuntimeInputOn(true).build(),
		})

		require.NotEmpty(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, true, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{newPipelineBuilder().build()})

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("marshaling", func(t *testing.T) {
		expected := `receivers:
    kubeletstats:
        collection_interval: 30s
        auth_type: serviceAccount
        endpoint: https://${env:MY_NODE_NAME}:10250
        insecure_skip_verify: true
exporters:
    otlp:
        endpoint: metrics.telemetry-system.svc.cluster.local:4317
        tls:
            insecure: true
        sending_queue:
            enabled: true
            queue_size: 512
        retry_on_failure:
            enabled: true
            initial_interval: 5s
            max_interval: 30s
            max_elapsed_time: 300s
processors: {}
extensions:
    health_check:
        endpoint: ${MY_POD_IP}:13133
service:
    pipelines:
        metrics:
            receivers:
                - kubeletstats
            processors: []
            exporters:
                - otlp
    telemetry:
        metrics:
            address: ${MY_POD_IP}:8888
        logs:
            level: info
    extensions:
        - health_check
`

		collectorConfig := makeAgentConfig(gatewayServiceName, []v1alpha1.MetricPipeline{newPipelineBuilder().withRuntimeInputOn(true).build()})

		yamlBytes, err := yaml.Marshal(collectorConfig)
		require.NoError(t, err)
		require.Equal(t, expected, string(yamlBytes))
	})
}
