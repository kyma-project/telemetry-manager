package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildAgentConfig(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "logs", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("receivers", func(t *testing.T) {
		t.Run("filelog receiver", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test").
					WithApplicationInput(true).WithKeepOriginalBody(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
			}, BuildOptions{AgentNamespace: "kyma-system"})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Receivers, "filelog/test")

			fileLogReceiver := collectorConfig.Receivers["filelog/test"]
			require.Equal(t, []string{"/var/log/pods/kyma-system_telemetry-log-agent*/*/*.log", "/var/log/pods/kyma-system_telemetry-fluent-bit*/*/*.log"}, fileLogReceiver.FileLog.Exclude)
			require.Equal(t, []string{"/var/log/pods/*/*/*.log"}, fileLogReceiver.FileLog.Include)
			require.False(t, fileLogReceiver.FileLog.IncludeFileName)
			require.True(t, fileLogReceiver.FileLog.IncludeFilePath)
			require.Equal(t, "beginning", fileLogReceiver.FileLog.StartAt)
			require.Equal(t, "file_storage", fileLogReceiver.FileLog.Storage)
			require.Equal(t, config.RetryOnFailure{
				Enabled:         true,
				InitialInterval: initialInterval,
				MaxInterval:     maxInterval,
				MaxElapsedTime:  maxElapsedTime,
			}, fileLogReceiver.FileLog.RetryOnFailure)
		})
	})

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, envVars, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").
				WithApplicationInput(true).WithKeepOriginalBody(true).
				WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build()}, BuildOptions{})

		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		const endpointEnvVar = "OTLP_ENDPOINT_TEST"
		expectedEndpoint := fmt.Sprintf("${%s}", endpointEnvVar)

		otlpExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, otlpExporterConfig.OTLP.Endpoint)

		require.Contains(t, envVars, endpointEnvVar)
		require.Equal(t, "http://localhost", string(envVars[endpointEnvVar]))
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test").
					WithApplicationInput(true).WithKeepOriginalBody(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build()}, BuildOptions{})
			require.NoError(t, err)

			actualExporterConfig := collectorConfig.Exporters["otlp/test"]
			require.True(t, actualExporterConfig.OTLP.TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").
				WithApplicationInput(true).WithKeepOriginalBody(true).
				WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
		}, BuildOptions{})

		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")

		require.NotEmpty(t, t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")

		require.NotEmpty(t, collectorConfig.Extensions.FileStorage)
		require.Contains(t, collectorConfig.Service.Extensions, "file_storage")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").
				WithApplicationInput(true).WithKeepOriginalBody(true).
				WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
		}, BuildOptions{})

		require.NoError(t, err)

		metricreaders := []config.MetricReader{
			{
				Pull: config.PullMetricReader{
					Exporter: config.MetricExporter{
						Prometheus: config.PrometheusMetricExporter{
							Host: "${MY_POD_IP}",
							Port: ports.Metrics,
						},
					},
				},
			},
		}

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "json", collectorConfig.Service.Telemetry.Logs.Encoding)
		require.Equal(t, metricreaders, collectorConfig.Service.Telemetry.Metrics.Readers)
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		t.Run("application log input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithName("test").
					WithApplicationInput(true).WithKeepOriginalBody(true).
					WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build()}, BuildOptions{})
			require.NoError(t, err)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "logs/test")

			require.Contains(t, collectorConfig.Service.Pipelines["logs/test"].Receivers, "filelog/test")
			require.Equal(t, []string{"memory_limiter", "transform/set-instrumentation-scope-runtime", "k8sattributes", "resource/insert-cluster-attributes", "resource/drop-kyma-attributes"}, collectorConfig.Service.Pipelines["logs/test"].Processors)
			require.Equal(t, []string{"otlp/test"}, collectorConfig.Service.Pipelines["logs/test"].Exporters)
		})
	})

	t.Run("multiple pipelines topology", func(t *testing.T) {
		t.Run("two logpipelines defined", func(t *testing.T) {
			pipleine1 := testutils.NewLogPipelineBuilder().WithName("test1").
				WithApplicationInput(true).WithKeepOriginalBody(true).
				WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build()

			pipleine2 := testutils.NewLogPipelineBuilder().WithName("test2").
				WithApplicationInput(true).WithKeepOriginalBody(true).
				WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build()

			collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{pipleine1, pipleine2}, BuildOptions{})
			require.NoError(t, err)

			require.Len(t, collectorConfig.Service.Pipelines, 2)
			require.Contains(t, collectorConfig.Service.Pipelines, "logs/test1")
			require.Contains(t, collectorConfig.Service.Pipelines, "logs/test2")

			require.Contains(t, collectorConfig.Service.Pipelines["logs/test1"].Receivers, "filelog/test1")
			require.Contains(t, collectorConfig.Service.Pipelines["logs/test2"].Receivers, "filelog/test2")

			require.Equal(t, []string{"memory_limiter", "transform/set-instrumentation-scope-runtime", "k8sattributes", "resource/insert-cluster-attributes", "resource/drop-kyma-attributes"}, collectorConfig.Service.Pipelines["logs/test1"].Processors)
			require.Equal(t, []string{"memory_limiter", "transform/set-instrumentation-scope-runtime", "k8sattributes", "resource/insert-cluster-attributes", "resource/drop-kyma-attributes"}, collectorConfig.Service.Pipelines["logs/test2"].Processors)

			require.Contains(t, collectorConfig.Service.Pipelines["logs/test1"].Exporters, "otlp/test1")
			require.Contains(t, collectorConfig.Service.Pipelines["logs/test2"].Exporters, "otlp/test2")
		})
	})
	t.Run("marshaling", func(t *testing.T) {
		goldenFileName := "config.yaml"

		collectorConfig, _, err := sut.Build(t.Context(), []telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithName("test").
				WithApplicationInput(true).WithKeepOriginalBody(true).
				WithOTLPOutput(testutils.OTLPEndpoint("http://localhost")).Build(),
		}, BuildOptions{InstrumentationScopeVersion: "main", AgentNamespace: "kyma-system", CloudProvider: "azure", ClusterName: "test-cluster"})
		require.NoError(t, err)
		configYAML, err := yaml.Marshal(collectorConfig)
		require.NoError(t, err, "failed to marshal config")

		goldenFilePath := filepath.Join("testdata", goldenFileName)
		goldenFile, err := os.ReadFile(goldenFilePath)
		require.NoError(t, err, "failed to load golden file")

		require.Equal(t, string(goldenFile), string(configYAML))
	})
}
