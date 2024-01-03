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
	"github.com/kyma-project/telemetry-manager/internal/namespaces"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMakeConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		expectedEndpoint := fmt.Sprintf("${%s}", "OTLP_ENDPOINT_TEST")
		require.Contains(t, collectorConfig.Exporters, "otlp/test")

		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.Equal(t, expectedEndpoint, actualExporterConfig.OTLP.Endpoint)
	})

	t.Run("secure", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test")
		actualExporterConfig := collectorConfig.Exporters["otlp/test"]
		require.False(t, actualExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("insecure", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-insecure").WithEndpoint("http://localhost").Build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-insecure")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-insecure"]
		require.True(t, actualExporterConfig.OTLP.TLS.Insecure)
	})

	t.Run("basic auth", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-basic-auth").WithBasicAuth("user", "password").Build(),
		})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-basic-auth")
		actualExporterConfig := collectorConfig.Exporters["otlp/test-basic-auth"]
		headers := actualExporterConfig.OTLP.Headers

		authHeader, existing := headers["Authorization"]
		require.True(t, existing)
		require.Equal(t, "${BASIC_AUTH_HEADER_TEST_BASIC_AUTH}", authHeader)
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.NotEmpty(t, collectorConfig.Extensions.Pprof.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()})
		require.NoError(t, err)

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "json", collectorConfig.Service.Telemetry.Logs.Encoding)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithName("test").Build()})
		require.NoError(t, err)
		require.Equal(t, 256, collectorConfig.Exporters["otlp/test"].OTLP.SendingQueue.QueueSize, "Pipeline should have the full queue size")
	})

	t.Run("multi pipeline queue size", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-1").Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-2").Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-3").Build()},
		)
		require.NoError(t, err)
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-1"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-2"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
		require.Equal(t, 85, collectorConfig.Exporters["otlp/test-3"].OTLP.SendingQueue.QueueSize, "Queue size should be divided by the number of pipelines")
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		t.Run("with no inputs enabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").OtlpInput(false).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"filter/drop-if-input-source-otlp",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with prometheus input enabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").PrometheusInput(true).PrometheusInputDiagnosticMetrics(true).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with prometheus input enabled and diagnostic metrics disabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").PrometheusInput(true).PrometheusInputDiagnosticMetrics(false).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"filter/drop-diagnostic-metrics-if-input-source-prometheus",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with prometheus input enabled and diagnostic metrics implicitly disabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").PrometheusInput(true).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-istio",
				"filter/drop-diagnostic-metrics-if-input-source-prometheus",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with runtime input enabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").RuntimeInput(true).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with istio input enabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").IstioInput(true).IstioInputDiagnosticMetrics(true).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with istio input enabled and diagnostic metrics disabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").IstioInput(true).IstioInputDiagnosticMetrics(false).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-diagnostic-metrics-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with istio input enabled and diagnostic metrics implicitly disabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").IstioInput(true).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-diagnostic-metrics-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with otlp input implicitly enabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})

		t.Run("with otlp input explicitly enabled", func(t *testing.T) {
			collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithName("test").OtlpInput(true).Build()},
			)
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Exporters, "otlp/test")

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Exporters, "otlp/test")
			require.Contains(t, collectorConfig.Service.Pipelines["metrics/test"].Receivers, "otlp")
			require.Equal(t, []string{"memory_limiter",
				"k8sattributes",
				"filter/drop-if-input-source-runtime",
				"filter/drop-if-input-source-prometheus",
				"filter/drop-if-input-source-istio",
				"resource/insert-cluster-name",
				"transform/resolve-service-name",
				"resource/drop-kyma-attributes",
				"batch",
			}, collectorConfig.Service.Pipelines["metrics/test"].Processors)
		})
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		collectorConfig, _, err := MakeConfig(ctx, fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test-1").RuntimeInput(true, testutils.ExcludeNamespaces(namespaces.System()...)).Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-2").PrometheusInput(true, testutils.ExcludeNamespaces(namespaces.System()...)).Build(),
			testutils.NewMetricPipelineBuilder().WithName("test-3").IstioInput(true).Build()},
		)
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Exporters, "otlp/test-3")

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Exporters, "otlp/test-1")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-1"].Receivers, "otlp")
		require.Equal(t, []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-if-input-source-prometheus",
			"filter/drop-if-input-source-istio",
			"filter/test-1-filter-by-namespace-runtime-input",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"resource/drop-kyma-attributes",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-1"].Processors)

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Exporters, "otlp/test-2")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-2"].Receivers, "otlp")
		require.Equal(t, []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-if-input-source-runtime",
			"filter/drop-if-input-source-istio",
			"filter/test-2-filter-by-namespace-prometheus-input",
			"filter/drop-diagnostic-metrics-if-input-source-prometheus",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"resource/drop-kyma-attributes",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-2"].Processors)

		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/test-3")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-3"].Exporters, "otlp/test-3")
		require.Contains(t, collectorConfig.Service.Pipelines["metrics/test-3"].Receivers, "otlp")
		require.Equal(t, []string{"memory_limiter",
			"k8sattributes",
			"filter/drop-if-input-source-runtime",
			"filter/drop-if-input-source-prometheus",
			"filter/drop-diagnostic-metrics-if-input-source-istio",
			"resource/insert-cluster-name",
			"transform/resolve-service-name",
			"resource/drop-kyma-attributes",
			"batch",
		}, collectorConfig.Service.Pipelines["metrics/test-3"].Processors)
	})

	t.Run("marshaling", func(t *testing.T) {
		config, _, err := MakeConfig(context.Background(), fakeClient, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithName("test").Build(),
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
