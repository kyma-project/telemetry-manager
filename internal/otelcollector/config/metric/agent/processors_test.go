package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestProcessors(t *testing.T) {
	t.Run("delete service name", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
		require.Len(t, collectorConfig.Processors.DeleteServiceName.Attributes, 1)
		require.Equal(t, "delete", collectorConfig.Processors.DeleteServiceName.Attributes[0].Action)
		require.Equal(t, "service.name", collectorConfig.Processors.DeleteServiceName.Attributes[0].Key)
	})

	t.Run("memory limiter proessor", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.MemoryLimiter)
		require.Equal(t, collectorConfig.Processors.MemoryLimiter.LimitPercentage, 75)
		require.Equal(t, collectorConfig.Processors.MemoryLimiter.SpikeLimitPercentage, 20)
		require.Equal(t, collectorConfig.Processors.MemoryLimiter.CheckInterval, "0.1s")
	})

	t.Run("batch processor", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.Batch)
		require.Equal(t, collectorConfig.Processors.Batch.SendBatchSize, 1024)
		require.Equal(t, collectorConfig.Processors.Batch.SendBatchMaxSize, 1024)
		require.Equal(t, collectorConfig.Processors.Batch.Timeout, "10s")
	})

	t.Run("insert input source runtime", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
		require.Len(t, collectorConfig.Processors.DeleteServiceName.Attributes, 1)
		require.Equal(t, "delete", collectorConfig.Processors.DeleteServiceName.Attributes[0].Action)
		require.Equal(t, "service.name", collectorConfig.Processors.DeleteServiceName.Attributes[0].Key)
	})

	t.Run("insert input source runtime", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
		require.Len(t, collectorConfig.Processors.InsertInputSourceRuntime.Attributes, 1)
		require.Equal(t, "insert", collectorConfig.Processors.InsertInputSourceRuntime.Attributes[0].Action)
		require.Equal(t, "kyma.source", collectorConfig.Processors.InsertInputSourceRuntime.Attributes[0].Key)
		require.Equal(t, "runtime", collectorConfig.Processors.InsertInputSourceRuntime.Attributes[0].Value)
	})

	t.Run("insert input source prometheus", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).PrometheusInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)
		require.Len(t, collectorConfig.Processors.InsertInputSourcePrometheus.Attributes, 1)
		require.Equal(t, "insert", collectorConfig.Processors.InsertInputSourcePrometheus.Attributes[0].Action)
		require.Equal(t, "kyma.source", collectorConfig.Processors.InsertInputSourcePrometheus.Attributes[0].Key)
		require.Equal(t, "prometheus", collectorConfig.Processors.InsertInputSourcePrometheus.Attributes[0].Value)
	})

	t.Run("insert input source istio", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().RuntimeInput(true).IstioInput(true).Build(),
		}, false)

		require.NotNil(t, collectorConfig.Processors.InsertInputSourceIstio)
		require.Len(t, collectorConfig.Processors.InsertInputSourceIstio.Attributes, 1)
		require.Equal(t, "insert", collectorConfig.Processors.InsertInputSourceIstio.Attributes[0].Action)
		require.Equal(t, "kyma.source", collectorConfig.Processors.InsertInputSourceIstio.Attributes[0].Key)
		require.Equal(t, "istio", collectorConfig.Processors.InsertInputSourceIstio.Attributes[0].Value)
	})
}
