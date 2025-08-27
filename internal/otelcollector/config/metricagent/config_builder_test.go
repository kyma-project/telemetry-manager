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
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
		require.NoError(t, err)
		actualExporterConfig := collectorConfig.Exporters["otlp"]
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", actualExporterConfig.(*common.OTLPExporter).Endpoint)
	})

	t.Run("insecure", func(t *testing.T) {
		t.Run("otlp exporter endpoint", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
			require.NoError(t, err)

			actualExporterConfig := collectorConfig.Exporters["otlp"]
			require.True(t, actualExporterConfig.(*common.OTLPExporter).TLS.Insecure)
		})
	})

	t.Run("extensions", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
		require.NoError(t, err)

		require.NotEmpty(t, collectorConfig.Extensions.HealthCheck.Endpoint)
		require.Contains(t, collectorConfig.Service.Extensions, "health_check")
		require.Contains(t, collectorConfig.Service.Extensions, "pprof")
		require.Contains(t, collectorConfig.Service.Extensions, "k8s_leader_elector")
	})

	t.Run("telemetry", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
		require.NoError(t, err)

		require.NotNil(t, collectorConfig.Service.Telemetry)
		require.NotNil(t, collectorConfig.Service.Telemetry.Metrics)
		require.Len(t, collectorConfig.Service.Telemetry.Metrics.Readers, 0)
	})

	t.Run("batch processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Processors, "batch")
		batchProcessor := collectorConfig.Processors["batch"].(*common.BatchProcessor)
		require.Equal(t, 1024, batchProcessor.SendBatchSize)
		require.Equal(t, "10s", batchProcessor.Timeout)
		require.Equal(t, 1024, batchProcessor.SendBatchMaxSize)
	})

	t.Run("memory limiter processor", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Processors, "memory_limiter")
		memoryLimiterProcessor := collectorConfig.Processors["memory_limiter"].(*common.MemoryLimiter)
		require.Equal(t, "1s", memoryLimiterProcessor.CheckInterval)
		require.Equal(t, 75, memoryLimiterProcessor.LimitPercentage)
		require.Equal(t, 15, memoryLimiterProcessor.SpikeLimitPercentage)
	})

	t.Run("otlp exporter", func(t *testing.T) {
		collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build()}, BuildOptions{})
		require.NoError(t, err)

		require.Contains(t, collectorConfig.Exporters, "otlp")
		otlpExporter := collectorConfig.Exporters["otlp"].(*common.OTLPExporter)
		require.Equal(t, "metrics.telemetry-system.svc.cluster.local:4317", otlpExporter.Endpoint)
		require.True(t, otlpExporter.TLS.Insecure)
		require.Equal(t, 512, otlpExporter.SendingQueue.QueueSize)
	})

	t.Run("volume metrics scraping disabled", func(t *testing.T) {
		tests := []struct {
			name                 string
			pipeline             telemetryv1alpha1.MetricPipeline
			volumeMetricsEnabled bool
		}{
			{
				name:                 "volume metrics enabled",
				pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputVolumeMetrics(true).Build(),
				volumeMetricsEnabled: true,
			},
			{
				name:                 "volume metrics disabled",
				pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputVolumeMetrics(false).Build(),
				volumeMetricsEnabled: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{tt.pipeline}, BuildOptions{})
				require.NoError(t, err)

				kubeletStatsReceiver := collectorConfig.Receivers["kubeletstats"].(*KubeletStatsReceiver)

				if tt.volumeMetricsEnabled {
					require.Contains(t, kubeletStatsReceiver.MetricGroups, MetricGroupTypeVolume)
					require.NotContains(t, collectorConfig.Service.Pipelines["metrics/runtime"].Processors, "filter/drop-non-pvc-volumes-metrics")
				} else {
					require.NotContains(t, kubeletStatsReceiver.MetricGroups, MetricGroupTypeVolume)
					require.NotContains(t, collectorConfig.Processors, "filter/drop-non-pvc-volumes-metrics")
				}
			})
		}
	})

	t.Run("pipelines", func(t *testing.T) {
		t.Run("with istio enabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(true).Build(),
			}, BuildOptions{
				IstioEnabled:                true,
				IstioCertPath:               "/etc/istio-output-certs",
				InstrumentationScopeVersion: "main",
			})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"k8s_cluster", "kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
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

		t.Run("with istio disabled", func(t *testing.T) {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{
				testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(false).Build(),
			}, BuildOptions{
				IstioEnabled:                false,
				IstioCertPath:               "/etc/istio-output-certs",
				InstrumentationScopeVersion: "main",
			})
			require.NoError(t, err)

			require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
			require.Equal(t, []string{"k8s_cluster", "kubeletstats"}, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers)
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
			pipeline            telemetryv1alpha1.MetricPipeline
			buildOptions        BuildOptions
			overwriteGoldenFile bool
		}{
			{
				name:           "basic runtime and prometheus input enabled",
				goldenFileName: "basic-runtime-prometheus.yaml",
				pipeline:       testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                false,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "istio input enabled",
				goldenFileName: "istio-enabled.yaml",
				pipeline:       testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(true).Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                true,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "istio input disabled",
				goldenFileName: "istio-disabled.yaml",
				pipeline:       testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(false).Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                false,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "runtime input only",
				goldenFileName: "runtime-only.yaml",
				pipeline:       testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(false).WithIstioInput(false).Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                false,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "prometheus input only",
				goldenFileName: "prometheus-only.yaml",
				pipeline:       testutils.NewMetricPipelineBuilder().WithRuntimeInput(false).WithPrometheusInput(true).WithIstioInput(false).Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                false,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "istio input with envoy metrics enabled",
				goldenFileName: "istio-envoy-metrics.yaml",
				pipeline:       testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).WithIstioInput(true).WithIstioInputEnvoyMetrics(true).Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                true,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "runtime input with specific resource metrics disabled",
				goldenFileName: "runtime-resources-some-disabled.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputVolumeMetrics(false).
					WithPrometheusInput(false).
					WithIstioInput(false).
					Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                false,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
			{
				name:           "comprehensive setup with all inputs enabled",
				goldenFileName: "setup-comprehensive.yaml",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputNodeMetrics(true).
					WithRuntimeInputVolumeMetrics(true).
					WithRuntimeInputStatefulSetMetrics(true).
					WithRuntimeInputDeploymentMetrics(true).
					WithRuntimeInputDaemonSetMetrics(true).
					WithRuntimeInputJobMetrics(true).
					WithPrometheusInput(true).
					WithPrometheusInputDiagnosticMetrics(true).
					WithIstioInput(true).
					WithIstioInputEnvoyMetrics(true).
					Build(),
				buildOptions: BuildOptions{
					IstioEnabled:                true,
					IstioCertPath:               "/etc/istio-output-certs",
					InstrumentationScopeVersion: "main",
				},
				overwriteGoldenFile: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config, _, err := sut.Build(ctx, []telemetryv1alpha1.MetricPipeline{tt.pipeline}, tt.buildOptions)
				require.NoError(t, err)

				configYAML, err := yaml.Marshal(config)
				require.NoError(t, err, "failed to marshal config")

				goldenFilePath := filepath.Join("testdata", tt.goldenFileName)
				if tt.overwriteGoldenFile {
					err = os.WriteFile(goldenFilePath, configYAML, 0600)
					require.NoError(t, err, "failed to overwrite golden file")

					t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", tt.goldenFileName)
				}

				goldenFile, err := os.ReadFile(goldenFilePath)
				require.NoError(t, err, "failed to load golden file")

				require.Equal(t, string(goldenFile), string(configYAML))
			})
		}
	})
}
