package agent

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func TestReceivers(t *testing.T) {
	t.Run("no pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().Build(),
			testutils.NewMetricPipelineBuilder().Build(),
		})

		require.Empty(t, collectorConfig.Receivers.KubeletStats)
	})

	t.Run("some pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(false).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
		})

		require.NotEmpty(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, true, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)
	})

	t.Run("all pipelines have runtime scraping enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
		})

		require.NotEmpty(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, true, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)
	})
}
