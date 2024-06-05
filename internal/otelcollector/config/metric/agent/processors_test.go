package agent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/internal/version"
)

func TestProcessors(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("delete service name", func(t *testing.T) {
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

	t.Run("insert input source runtime", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{})

		require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
		require.Len(t, collectorConfig.Processors.DeleteServiceName.Attributes, 1)
		require.Equal(t, "delete", collectorConfig.Processors.DeleteServiceName.Attributes[0].Action)
		require.Equal(t, "service.name", collectorConfig.Processors.DeleteServiceName.Attributes[0].Key)
	})

	t.Run("set instrumentation scope runtime", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{})
		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeRuntime)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopeRuntime.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Statements, 2)
		require.Equal(t, fmt.Sprintf("set(version, \"%v\") where name == \"otelcol/kubeletstatsreceiver\"", version.Version), collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/runtime\") where name == \"otelcol/kubeletstatsreceiver\"", collectorConfig.Processors.SetInstrumentationScopeRuntime.MetricStatements[0].Statements[1])
	})

	t.Run("set instrumentation scope prometheus", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithPrometheusInput(true).Build(),
		}, BuildOptions{})
		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopePrometheus)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopePrometheus.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Statements, 2)
		require.Equal(t, fmt.Sprintf("set(version, \"%v\") where name == \"otelcol/prometheusreceiver\"", version.Version), collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/prometheus\") where name == \"otelcol/prometheusreceiver\"", collectorConfig.Processors.SetInstrumentationScopePrometheus.MetricStatements[0].Statements[1])
	})

	t.Run("set instrumentation scope istio", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).WithIstioInput(true).Build(),
		}, BuildOptions{})
		require.NotNil(t, collectorConfig.Processors.SetInstrumentationScopeIstio)
		require.Equal(t, "ignore", collectorConfig.Processors.SetInstrumentationScopeIstio.ErrorMode)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements, 1)
		require.Equal(t, "scope", collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Context)
		require.Len(t, collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Statements, 2)
		require.Equal(t, fmt.Sprintf("set(version, \"%v\") where name == \"otelcol/prometheusreceiver\"", version.Version), collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Statements[0])
		require.Equal(t, "set(name, \"io.kyma-project.telemetry/istio\") where name == \"otelcol/prometheusreceiver\"", collectorConfig.Processors.SetInstrumentationScopeIstio.MetricStatements[0].Statements[1])
	})

}
