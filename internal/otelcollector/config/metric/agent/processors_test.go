package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestProcessors(t *testing.T) {
	t.Run("delete service name", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).WithPrometheusInputOn(true).Build(),
		})

		require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
		require.Len(t, collectorConfig.Processors.DeleteServiceName.Attributes, 1)
		require.Equal(t, "delete", collectorConfig.Processors.DeleteServiceName.Attributes[0].Action)
		require.Equal(t, "service.name", collectorConfig.Processors.DeleteServiceName.Attributes[0].Key)
	})

	t.Run("insert input source runtime", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).WithPrometheusInputOn(true).Build(),
		})

		require.NotNil(t, collectorConfig.Processors.DeleteServiceName)
		require.Len(t, collectorConfig.Processors.DeleteServiceName.Attributes, 1)
		require.Equal(t, "delete", collectorConfig.Processors.DeleteServiceName.Attributes[0].Action)
		require.Equal(t, "service.name", collectorConfig.Processors.DeleteServiceName.Attributes[0].Key)
	})

	t.Run("insert input source runtime", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).WithPrometheusInputOn(true).Build(),
		})

		require.NotNil(t, collectorConfig.Processors.InsertInputSourceRuntime)
		require.Len(t, collectorConfig.Processors.InsertInputSourceRuntime.Attributes, 1)
		require.Equal(t, "insert", collectorConfig.Processors.InsertInputSourceRuntime.Attributes[0].Action)
		require.Equal(t, "kyma.source", collectorConfig.Processors.InsertInputSourceRuntime.Attributes[0].Key)
		require.Equal(t, "runtime", collectorConfig.Processors.InsertInputSourceRuntime.Attributes[0].Value)
	})

	t.Run("insert input source prometheus", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).WithPrometheusInputOn(true).Build(),
		})

		require.NotNil(t, collectorConfig.Processors.InsertInputSourcePrometheus)
		require.Len(t, collectorConfig.Processors.InsertInputSourcePrometheus.Attributes, 1)
		require.Equal(t, "insert", collectorConfig.Processors.InsertInputSourcePrometheus.Attributes[0].Action)
		require.Equal(t, "kyma.source", collectorConfig.Processors.InsertInputSourcePrometheus.Attributes[0].Key)
		require.Equal(t, "prometheus", collectorConfig.Processors.InsertInputSourcePrometheus.Attributes[0].Value)
	})
}
