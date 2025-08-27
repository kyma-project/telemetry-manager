package metricagent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestBuildConfig(t *testing.T) {
	ctx := context.Background()
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("otlp exporter endpoint", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})
		require.NoError(t, err)
		actualExporterConfig := collectorConfig.Exporters["otlp"]
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.(*common.OTLPExporter).Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})
			require.NoError(t, err)

			actualExporterConfig := collectorConfig.Exporters["otlp"]
			require.True(t, actualExporterConfig.(*common.OTLPExporter).TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().Build()}, BuildOptions{})
		require.NoError(t, err)

		metricreaders := []common.MetricReader{
			{
				Pull: common.PullMetricReader{
					Exporter: common.MetricExporter{
						Prometheus: common.PrometheusMetricExporter{
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
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.NotContains(t, collectorConfig.Processors, "resource/delete-service-name")

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("runtime enabled with different resources", func(t *testing.T) {
			tt := []struct {
				name                 string
				pipeline             telemetryv1alpha1.MetricPipeline
				volumeMetricsEnabled bool
			}{
				{
					name:                 "runtime enabled with default metrics enabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
					volumeMetricsEnabled: true,
				}, {
					name:                 "runtime enabled with and only pod metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputPodMetrics(false).Build(),
					volumeMetricsEnabled: true,
				}, {
					name:                 "runtime enabled with and only container metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputContainerMetrics(false).Build(),
					volumeMetricsEnabled: true,
				}, {
					name:                 "runtime enabled with and only node metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputNodeMetrics(false).Build(),
					volumeMetricsEnabled: true,
				},
				{
					name:                 "runtime enabled with and only volume metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputVolumeMetrics(false).Build(),
					volumeMetricsEnabled: false,
				}, {
					name:                 "runtime enabled with only statefulset metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputStatefulSetMetrics(false).Build(),
					volumeMetricsEnabled: true,
				}, {
					name:                 "runtime enabled with only daemonset metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputDaemonSetMetrics(false).Build(),
					volumeMetricsEnabled: true,
				}, {
					name:                 "runtime enabled with only deployment metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputDeploymentMetrics(false).Build(),
					volumeMetricsEnabled: true,
				}, {
					name:                 "runtime enabled with only job metrics disabled",
					pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputJobMetrics(false).Build(),
					volumeMetricsEnabled: true,
				},
			}
			for _, tc := range tt {
				expectedReceiverIDs := []string{"kubeletstats", "k8s_cluster"}
				expectedExporterIDs := []string{"otlp"}

				var expectedProcessorIDs []string
				if tc.volumeMetricsEnabled {
					expectedProcessorIDs = []string{"memory_limiter", "filter/drop-non-pvc-volumes-metrics", "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}
				} else {
					expectedProcessorIDs = []string{"memory_limiter", "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}
				}

				t.Run(tc.name, func(t *testing.T) {
					collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{tc.pipeline}, BuildOptions{})
					require.NoError(t, err)

					require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
					require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
					require.Contains(t, collectorConfig.Processors, "transform/insert-skip-enrichment-attribute")
					require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
					require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")

					if tc.volumeMetricsEnabled {
						require.Contains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")
					} else {
						require.NotContains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")
					}

					require.Len(t, collectorConfig.Service.Pipelines, 1)
					require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
					require.Equal(t, expectedReceiverIDs, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
					require.Equal(t, expectedProcessorIDs, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
					require.Equal(t, expectedExporterIDs, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
				})
			}
		})

		t.Run("prometheus input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("istio input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithIstioInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"memory_limiter", "istio_noise_filter", "resource/delete-service-name", "transform/set-instrumentation-scope-istio", "batch"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})

		t.Run("multiple input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")
			require.Contains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")

			require.Len(t, collectorConfig.Service.Pipelines, 3)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-non-pvc-volumes-metrics", "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/istio")
			require.Equal(t, []string{"prometheus/istio"}, collectorConfig.Service.Pipelines["metrics/istio"].Receivers)
			require.Equal(t, []string{"memory_limiter", "istio_noise_filter", "resource/delete-service-name", "transform/set-instrumentation-scope-istio", "batch"}, collectorConfig.Service.Pipelines["metrics/istio"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/istio"].Exporters)
		})
	})

	t.Run("multi pipeline topology", func(t *testing.T) {
		t.Run("no pipeline has input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().Build(),
				testutils.NewMetricPipelineBuilder().Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.NotContains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")

			require.Len(t, collectorConfig.Service.Pipelines, 0)
		})

		t.Run("some pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(false).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")
			require.Contains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-non-pvc-volumes-metrics", "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("all pipelines have runtime input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")
			require.Contains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-non-pvc-volumes-metrics", "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters)
		})

		t.Run("some pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(false).Build(),
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-istio")

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("all pipelines have prometheus input enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.NotContains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")

			require.Len(t, collectorConfig.Service.Pipelines, 1)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/prometheus")
			require.Equal(t, []string{"prometheus/app-pods", "prometheus/app-services"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Receivers)
			require.Equal(t, []string{"memory_limiter", "resource/delete-service-name", "transform/set-instrumentation-scope-prometheus", "batch"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Processors)
			require.Equal(t, []string{"otlp"}, collectorConfig.Service.Pipelines["metrics/prometheus"].Exporters)
		})

		t.Run("multiple input types enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			}, BuildOptions{})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Processors, "resource/delete-service-name")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-runtime")
			require.Contains(t, collectorConfig.Processors, "transform/set-instrumentation-scope-prometheus")
			require.Contains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")

			require.Len(t, collectorConfig.Service.Pipelines, 2)
			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"kubeletstats", "k8s_cluster"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
			require.Equal(t, []string{"memory_limiter", "filter/drop-non-pvc-volumes-metrics", "filter/drop-virtual-network-interfaces", "resource/delete-service-name", "transform/set-instrumentation-scope-runtime", "transform/insert-skip-enrichment-attribute", "batch"}, collectorConfig.Service.Pipelines["metrics/runtime"].Processors)
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
				config, _, err := sut.Build(ctx, pipelines, BuildOptions{
					IstioEnabled:                tt.istioEnabled,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				})
				require.NoError(t, err)
				configYAML, err := yaml.Marshal(config)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				if tt.overwriteGoldenFile {
					err = os.WriteFile(goldenFilePath, configYAML, 0600)
					require.NoError(t, err, "failed to overwrite golden file")

					t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", goldenFilePath)

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
