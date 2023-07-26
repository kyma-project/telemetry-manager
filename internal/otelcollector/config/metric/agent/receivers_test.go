package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestReceivers(t *testing.T) {
	t.Run("no input enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().Build(),
		})

		require.Nil(t, collectorConfig.Receivers.KubeletStats)
		require.Nil(t, collectorConfig.Receivers.PrometheusSelf)
		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
	})

	t.Run("runtime input enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInputOn(true).Build(),
		})

		require.NotNil(t, collectorConfig.Receivers.KubeletStats)
		require.Equal(t, "serviceAccount", collectorConfig.Receivers.KubeletStats.AuthType)
		require.Equal(t, "https://${env:MY_NODE_NAME}:10250", collectorConfig.Receivers.KubeletStats.Endpoint)
		require.Equal(t, false, collectorConfig.Receivers.KubeletStats.InsecureSkipVerify)

		require.Nil(t, collectorConfig.Receivers.PrometheusSelf)
		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
	})

	t.Run("prometheus input enabled", func(t *testing.T) {
		collectorConfig := MakeConfig(types.NamespacedName{Name: "metrics-gateway"}, []v1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithPrometheusInputOn(true).Build(),
		})

		require.Nil(t, collectorConfig.Receivers.KubeletStats)
		require.NotNil(t, collectorConfig.Receivers.PrometheusSelf)
		require.Len(t, collectorConfig.Receivers.PrometheusSelf.Config.ScrapeConfigs, 1)
		require.Len(t, collectorConfig.Receivers.PrometheusSelf.Config.ScrapeConfigs[0].ServiceDiscoveryConfigs, 1)

		require.NotNil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Len(t, collectorConfig.Receivers.PrometheusAppPods.Config.ScrapeConfigs, 1)
		require.Len(t, collectorConfig.Receivers.PrometheusAppPods.Config.ScrapeConfigs[0].ServiceDiscoveryConfigs, 1)
	})
}
