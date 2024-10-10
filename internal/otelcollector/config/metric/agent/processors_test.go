package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestProcessors(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("delete service name processor", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{})

		require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
		require.Len(t, collectorConfig.Processors.DeleteServiceName.Attributes, 1)
		require.Equal(t, "delete", collectorConfig.Processors.DeleteServiceName.Attributes[0].Action)
		require.Equal(t, "service.name", collectorConfig.Processors.DeleteServiceName.Attributes[0].Key)
	})

	t.Run("memory limiter proessor", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{})

		require.NotNil(t, collectorConfig.Processors.MemoryLimiter)
		require.Equal(t, collectorConfig.Processors.MemoryLimiter.LimitPercentage, 75)
		require.Equal(t, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage, 15)
		require.Equal(t, collectorConfig.Processors.MemoryLimiter.CheckInterval, "1s")
	})

	t.Run("batch processor", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{})

		require.NotNil(t, collectorConfig.Processors.Batch)
		require.Equal(t, collectorConfig.Processors.Batch.SendBatchSize, 1024)
		require.Equal(t, collectorConfig.Processors.Batch.SendBatchMaxSize, 1024)
		require.Equal(t, collectorConfig.Processors.Batch.Timeout, "10s")
	})

	t.Run("set instrumentation scope runtime", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{
			InstrumentationScopeVersion: "main",
		})
		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopeRuntime.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Statements, 2)
		require.Equal(t, "set(version, \"main\") where name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver\"", collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/runtime\") where name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver\"", collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Statements[1])
	})

	t.Run("set instrumentation scope prometheus", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{
			InstrumentationScopeVersion: "main",
		})
		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopePrometheus.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Statements, 2)
		require.Equal(t, "set(version, \"main\") where name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"", collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/prometheus\") where name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"", collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Statements[1])
	})

	t.Run("set instrumentation scope istio", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithIstioInput(true).Build(),
		}, BuildOptions{
			InstrumentationScopeVersion: "main",
		})
		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopeIstio.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Statements, 2)
		require.Equal(t, "set(version, \"main\") where name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"", collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/istio\") where name == \"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver\"", collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Statements[1])
	})

	t.Run("insert skip enrichment attribute processor", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
		}, BuildOptions{})

		expectedInsertSkipEnrichmentAttributeProcessor := metric.TransformProcessor{
			ErrorMode: "ignore",
			MetricStatements: []config.TransformProcessorStatements{
				{
					Context:    "metric",
					Statements: []string{"set(resource.attributes[\"io.kyma-project.telemetry.skip_enrichment\"], \"true\")"},
					Conditions: []string{"IsMatch(name, \"^k8s.node.*\")"},
				},
			},
		}
		require.Equal(t, expectedInsertSkipEnrichmentAttributeProcessor, *collectorConfig.Processors.InsertSkipEnrichmentAttribute)
	})

	t.Run("drop non-PVC volumes metrics processor", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithRuntimeInputVolumeMetrics(true).Build(),
		}, BuildOptions{})

		expectedDropNonPVCVolumesMetricsProcessor := FilterProcessor{
			Metrics: FilterProcessorMetrics{
				Metric: []string{
					"resource.attributes[\"k8s.volume.name\"] != nil and resource.attributes[\"k8s.volume.type\"] != \"persistentVolumeClaim\"",
				},
			},
		}
		require.Equal(t, expectedDropNonPVCVolumesMetricsProcessor, *collectorConfig.Processors.DropNonPVCVolumesMetrics)
	})
}
