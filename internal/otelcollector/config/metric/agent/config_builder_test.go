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
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestBuildAgentConfig(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})
		actualExporterConfig := collectorConfig.Exporters.OTLP
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})

			actualExporterConfig := collectorConfig.Exporters.OTLP
			require.True(t, actualExporterConfig.TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})

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
		t.Run("no input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			}, BuildOptions{})

			require.Nil(t, collectorConfig.Processors.DeleteServiceName)

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("runtime input enabled with default resources metrics enabled ", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertSkipEnrichmentAttribute)
			require.Nil(t, collectorConfig.Processors.DropNonPVCVolumesMetrics)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("runtime input enabled with volume metrics enabled ", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputVolumeMetrics(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.NotNil(t, collectorConfig.Processors.InsertSkipEnrichmentAttribute)
			require.NotNil(t, collectorConfig.Processors.DropNonPVCVolumesMetrics)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-non-pvc-volumes-metrics", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("runtime enabled with different resources", func(t *testing.T) {
			tt := []struct {
				name     string
				pipeline telemetryv1alpha1.MetricPipeline
			}{
				{
					name:     "runtime enabled with default metrics",
					pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputPodMetrics(true).WithRuntimeInputContainerMetrics(true).Build(),
				}, {
					name:     "runtime enabled with node metrics",
					pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputNodeMetrics(true).Build(),
				}, {
					name:     "runtime enabled with statefulset metrics",
					pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputStatefulSetMetrics(true).Build(),
				}, {
					name:     "runtime enabled with daemonset metrics",
					pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputDaemonSetMetrics(true).Build(),
				}, {
					name:     "runtime enabled with deployment metrics",
					pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputDeploymentMetrics(true).Build(),
				}, {
					name:     "runtime enabled with job metrics",
					pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputJobMetrics(true).Build(),
				},
			}
			for _, tc := range tt {
				expectedReceiverIDs := []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}
				expectedProcessorIDs := []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}
				expectedExporterIDs := []string{"otlp"}

				t.Run(tc.name, func(t *testing.T) {
					collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{tc.pipeline}, BuildOptions{})

					require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
					require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
					require.NotNil(t, collectorConfig.Processors.InsertSkipEnrichmentAttribute)
					require.Nil(t, collectorConfig.Processors.DropNonPVCVolumesMetrics)
					require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
					require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

					require.Len(t, collectorConfig.Service.Pipelines, 1)
					require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
					require.Equal(t, expectedReceiverIDs, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
					require.Equal(t, expectedProcessorIDs, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
					require.Equal(t, expectedExporterIDs, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
				})
			}
		})

		t.Run("prometheus input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("istio input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithIstioInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-internal-communication", "resource/delete-service-name", "transform/set-instrumentation-scope-istio", "batch"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})

		t.Run("multiple input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 3)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-internal-communication", "resource/delete-service-name", "transform/set-instrumentation-scope-istio", "batch"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		t.Run("no pipeline has input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
				testutils.NewMetricPipelineBuilder().Build(),
			}, BuildOptions{})

			require.Nil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("some pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(false).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("all pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("some pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(false).Build(),
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("all pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.Nil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("multiple input types enabled", func(t *testing.T) {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})

			require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
			require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)

			require.Len(t, collectorConfig.Service.Pipelines, 2)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "singleton_receiver_creator/k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})
	})

	t.Run("marshaling", func(t *testing.T) {
		tests := []struct {
			name                string
			goldenFileName      string
			istioEnabled        bool
			overwriteGoldenFile bool
		}{
			{
				name:           "istio not enabled",
				goldenFileName: "config_istio_not_enabled.yaml",
				istioEnabled:   false,
			},
			{
				name:           "istio enabled",
				goldenFileName: "config_istio_enabled.yaml",
				istioEnabled:   true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pipelines := []telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(tt.istioEnabled).Build(),
				}
				config := sut.Build(pipelines, BuildOptions{
					IstioEnabled:                tt.istioEnabled,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				})
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
