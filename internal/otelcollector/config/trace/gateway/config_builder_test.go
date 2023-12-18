package gateway

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMakeConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{testutils.NewTracePipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")
		require.Contains(t, collectorConfig.Exporters, "otlp/test")
		otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, otlpExporterConfig.OTLP.Endpoint)
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{testutils.NewTracePipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test")
		otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, otlpExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{
			testutils.NewTracePipelineBuilder().WithName("test-insecure").WithEndpoint("http://localhost").Build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{
			testutils.NewTracePipelineBuilder().WithName("test-basic-auth").WithBasicAuth("user", "password").Build(),
		})
		require.NoError(t, err)
		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")
		otlpExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := otlpExporterConfig.OTLP.Headers

		authHeader, existing := headers["Authorization"]
		require.True(t, existing)
		require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{testutils.NewTracePipelineBuilder().Build()})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.NotEmpty(t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{testutils.NewTracePipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "json", collectorConfig.Service.Telemetry.Logs.Encoding)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{testutils.NewTracePipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		require.Equal(t, 256, collectorConfig.Exporters["otlp/test"].OTLP.SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{
			testutils.NewTracePipelineBuilder().WithName("test-1").Build(),
			testutils.NewTracePipelineBuilder().WithName("test-2").Build(),
			testutils.NewTracePipelineBuilder().WithName("test-3").Build()},
		)
		require.NoError(t, err)
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-1"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-2"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-3"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.TracePipeline{testutils.NewTracePipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Service.Pipelines, "traces/test")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Receivers, "otlp")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Receivers, "opencensus")

		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[1], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[2], "filter/drop-noisy-spans")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[3], "resource/insert-cluster-name")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[4], "transform/resolve-service-name")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[5], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test"].Processors[6], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines["traces/test"].Exporters, "otlp/test")
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(context.Background(), fakeClient, []telemetryv1alpha1.TracePipeline{
			testutils.NewTracePipelineBuilder().WithName("test-1").Build(),
			testutils.NewTracePipelineBuilder().WithName("test-2").Build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")

		require.Contains(t, collectorConfig.Service.Pipelines, "traces/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test-1"].Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test-1"].Receivers, "otlp")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test-1"].Receivers, "opencensus")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[1], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[2], "filter/drop-noisy-spans")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[3], "resource/insert-cluster-name")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[4], "transform/resolve-service-name")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[5], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-1"].Processors[6], "batch")

		require.Contains(t, collectorConfig.Service.Pipelines, "traces/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test-2"].Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test-2"].Receivers, "otlp")
		require.Contains(t, collectorConfig.Service.Pipelines["traces/test-2"].Receivers, "opencensus")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[0], "memory_limiter")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[1], "k8sattributes")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[2], "filter/drop-noisy-spans")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[3], "resource/insert-cluster-name")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[4], "transform/resolve-service-name")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[5], "resource/drop-kyma-attributes")
		require.Equal(t, collectorConfig.Service.Pipelines["traces/test-2"].Processors[6], "batch")
	})

	t.Run("marshaling", func(t *testing.T) {
		config, _, err := MakeConfig(context.Background(), fakeClient, []telemetryv1alpha1.TracePipeline{
			testutils.NewTracePipelineBuilder().WithName("test").Build(),
		})
		require.NoError(t, err)

		configYAML, err := yaml.Marshal(config)
		require.NoError(t, err, "failed to marshal config")

		goldenFilePath := filepath.Join("testdata", "config.yaml")
		goldenFile, err := os.ReadFile(goldenFilePath)
		require.NoError(t, err, "failed to load golden file")

		require.NoError(t, err)
		require.Equal(t, string(goldenFile), string(configYAML))
	})
}
