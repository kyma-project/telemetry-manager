package gateway

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMakeConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")
		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, actualExporterConfig.Endpoint)
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")
		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, actualExporterConfig.TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-insecure").WithEndpoint("http://localhost").Build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-basic-auth").WithBasicAuth("user", "password").Build(),
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := actualExporterConfig.Headers

		authHeader, existing := headers["Authorization"]
		require.True(t, existing)
		require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.NotEmpty(t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		require.Equal(t, 256, collectorConfig.Exporters["otlp/test"].SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-3").Build()},
		)
		require.NoError(t, err)
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-1"].SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-2"].SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-3"].SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
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
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-2").Build()},
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

		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []v1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)

		yamlBytes, err := yaml.Marshal(collectorConfig)
		require.NoError(t, err)
		require.Equal(t, expected, string(yamlBytes))
	})
}
