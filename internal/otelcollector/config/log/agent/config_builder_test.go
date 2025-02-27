package agent

import (
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

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()}, BuildOptions{})
		actualExporterConfig := collectorConfig.Exporters.OTLP
		require.Equal(t, "logs.telemetry-system.svc.cluster.local:4317", actualExporterConfig.Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()}, BuildOptions{})

			actualExporterConfig := collectorConfig.Exporters.OTLP
			require.True(t, actualExporterConfig.TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()}, BuildOptions{})

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")

		require.NotEmpty(t, t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")

		require.NotEmpty(t, collectorConfig.Extensions.FileStorage)
		require.Contains(t, collectorConfig.Service.Extensions, "file_storage")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.LogPipeline{
			testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()}, BuildOptions{})

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
			collectorConfig := sut.Build([]telemetryv1alpha1.LogPipeline{
				testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()}, BuildOptions{})

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "logs")

			require.Equal(t, []string{"filelog"}, collectorConfig.Service.Pipelines["logs"].Receivers)
			require.Equal(t, []string{"memory_limiter", "transform/set-instrumentation-scope-runtime"}, collectorConfig.Service.Pipelines["logs"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["logs"].Exporters)
		})
	})
	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name              string
			goldenFileName    string
			withOTLPInput     bool
			compatibilityMode bool
		}{
			{
				name:              "Compatibility mode disabled",
				goldenFileName:    "config.yaml",
				compatibilityMode: false,
			},
			{
				name:              "Compatibility mode enabled",
				goldenFileName:    "config_compatibility_enabled.yaml",
				compatibilityMode: true,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				goldenFileName := tt.goldenFileName

				collectorConfig := sut.Build([]telemetryv1alpha1.LogPipeline{
					testutils.NewLogPipelineBuilder().WithApplicationInput(true).WithKeepOriginalBody(true).Build()}, BuildOptions{
					InstrumentationScopeVersion:     "main",
					AgentNamespace:                  "kyma-system",
					InternalMetricCompatibilityMode: tt.compatibilityMode,
				})
				configYAML, err := yaml.Marshal(collectorConfig)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", goldenFileName)
				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
