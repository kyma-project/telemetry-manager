package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestMakeAgentConfig(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig := MakeConfig(gatewayServiceName, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, false)

		actualExporterConfig := collectorConfig.Exporters.OTLP
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig := MakeConfig(gatewayServiceName, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, false)

			actualExporterConfig := collectorConfig.Exporters.OTLP
			require.True(t, actualExporterConfig.TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig := MakeConfig(gatewayServiceName, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, false)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig := MakeConfig(gatewayServiceName, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, false)

		require.Equal(t, "info", collectorConfig.Service.Telemetry.Logs.Level)
		require.Equal(t, "json", collectorConfig.Service.Telemetry.Logs.Encoding)
		require.Equal(t, "${MY_POD_IP}:8888", collectorConfig.Service.Telemetry.Metrics.Address)
	})

	t.Run("single pipeline topology", func(t *testing.T) {
		t.Run("no input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			}, false)

			require.Nil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("runtime input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().RuntimeInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-runtime", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("prometheus input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("istio input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().IstioInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-internal-communication", "resource/delete-service-name", "resource/insert-input-source-istio", "batch"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})

		t.Run("multiple input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).IstioInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 3)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-runtime", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-internal-communication", "resource/delete-service-name", "resource/insert-input-source-istio", "batch"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		t.Run("no pipeline has input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
				testutils.NewMetricPipelineBuilder().Build(),
			}, false)

			require.Nil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("some pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().RuntimeInput(false).Build(),
				testutils.NewMetricPipelineBuilder().RuntimeInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-runtime", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("all pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().RuntimeInput(true).Build(),
				testutils.NewMetricPipelineBuilder().RuntimeInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.Nil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-runtime", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("some pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().PrometheusInput(false).Build(),
				testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("all pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build(),
				testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("multiple input types enabled", func(t *testing.T) {
			collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().PrometheusInput(true).Build(),
				testutils.NewMetricPipelineBuilder().RuntimeInput(true).Build(),
			}, false)

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 2)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-runtime", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "resource/insert-input-source-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})
	})

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name                string
			goldenFileName      string
			istioActive         bool
			overwriteGoldenFile bool
		}{
			{
				name:           "istio not active",
				goldenFileName: "config_istio_not_active.yaml",
			},
			{
				name:           "istio active",
				goldenFileName: "config_istio_active.yaml",
				istioActive:    true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipelines := []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).IstioInput(true).Build(),
				}
				config := MakeConfig(gatewayServiceName, pipelines, tt.istioActive)
				configYAML, err := yaml.Marshal(config)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				if tt.overwriteGoldenFile {
					err = os.WriteFile(goldenFilePath, configYAML, 0600)
					require.NoError(t, err, "failed to overwrite golden file")
					return
				}

				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.NoError(t, err)
				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
