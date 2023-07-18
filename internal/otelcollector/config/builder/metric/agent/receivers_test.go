package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestReceivers(t *testing.T) {
	t.Run("no pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().Build(),
			testutils.NewMetricPipelineBuilder().Build(),
		})

		require.Empty(t, collectorConfig.Receivers.KubeletStats)
		require.Len(t, collectorConfig.Service.Pipelines, 0)
	})

	t.Run("some pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(false).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
		})

		require.NotEmpty(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, false, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)

		require.Len(t, collectorConfig.Service.Pipelines, 1)
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers, []string{"kubeletstats"})
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/runtime"].Processors, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"})
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters, []string{"otlp"})
	})

	t.Run("all pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
		})

		require.NotEmpty(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, false, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)

		require.Len(t, collectorConfig.Service.Pipelines, 1)
		require.Contains(t, collectorConfig.Service.Pipelines, "metrics/runtime")
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/runtime"].Receivers, []string{"kubeletstats"})
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/runtime"].Processors, []string{"resource/delete-service-name", "resource/insert-input-source-runtime"})
		require.Equal(t, collectorConfig.Service.Pipelines["metrics/runtime"].Exporters, []string{"otlp"})
	})
}
