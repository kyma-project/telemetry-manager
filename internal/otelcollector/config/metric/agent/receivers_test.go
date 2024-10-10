package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

func TestReceivers(t *testing.T) {
	gatewayServiceName := types.NamespacedName{Name: "metrics", Namespace: "telemetry-system"}
	sut := Builder{
		Config: BuilderConfig{
			GatewayOTLPServiceName: gatewayServiceName,
		},
	}

	t.Run("no input enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().Build(),
		}, BuildOptions{})

		require.Nil(t, collectorConfig.Receivers.KubeletStats)
		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Nil(t, collectorConfig.Receivers.PrometheusIstio)
	})

	t.Run("runtime input enabled with default resources metrics enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
		}, BuildOptions{})

		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

		expectedKubeletStatsReceiver := KubeletStatsReceiver{
			CollectionInterval: "30s",
			AuthType:           "serviceAccount",
			Endpoint:           "https://${MY_NODE_NAME}:10250",
			InsecureSkipVerify: true,
			MetricGroups:       []MetricGroupType{"container", "pod"},
			Metrics: KubeletStatsMetricsConfig{
				ContainerCPUUsage:            MetricConfig{Enabled: true},
				ContainerCPUUtilization:      MetricConfig{Enabled: false},
				K8sPodCPUUsage:               MetricConfig{Enabled: true},
				K8sPodCPUUtilization:         MetricConfig{Enabled: false},
				K8sNodeCPUUsage:              MetricConfig{Enabled: true},
				K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
				K8sNodeCPUTime:               MetricConfig{Enabled: false},
				K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
				K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
				K8sNodeNetworkIO:             MetricConfig{Enabled: false},
				K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
			},
			ExtraMetadataLabels: []string{
				"k8s.volume.type",
			},
		}
		require.Equal(t, expectedKubeletStatsReceiver, *collectorConfig.Receivers.KubeletStats)
	})

	t.Run("runtime input enabled with only pod metrics enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(true).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				Build(),
		}, BuildOptions{})

		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

		expectedKubeletStatsReceiver := KubeletStatsReceiver{
			CollectionInterval: "30s",
			AuthType:           "serviceAccount",
			Endpoint:           "https://${MY_NODE_NAME}:10250",
			InsecureSkipVerify: true,
			MetricGroups:       []MetricGroupType{"pod"},
			Metrics: KubeletStatsMetricsConfig{
				ContainerCPUUsage:            MetricConfig{Enabled: true},
				ContainerCPUUtilization:      MetricConfig{Enabled: false},
				K8sPodCPUUsage:               MetricConfig{Enabled: true},
				K8sPodCPUUtilization:         MetricConfig{Enabled: false},
				K8sNodeCPUUsage:              MetricConfig{Enabled: true},
				K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
				K8sNodeCPUTime:               MetricConfig{Enabled: false},
				K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
				K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
				K8sNodeNetworkIO:             MetricConfig{Enabled: false},
				K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
			},
			ExtraMetadataLabels: []string{
				"k8s.volume.type",
			},
		}
		require.Equal(t, expectedKubeletStatsReceiver, *collectorConfig.Receivers.KubeletStats)
	})

	t.Run("runtime input enabled with only container metrics enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				Build(),
		}, BuildOptions{})

		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

		expectedKubeletStatsReceiver := KubeletStatsReceiver{
			CollectionInterval: "30s",
			AuthType:           "serviceAccount",
			Endpoint:           "https://${MY_NODE_NAME}:10250",
			InsecureSkipVerify: true,
			MetricGroups:       []MetricGroupType{"container"},
			Metrics: KubeletStatsMetricsConfig{
				ContainerCPUUsage:            MetricConfig{Enabled: true},
				ContainerCPUUtilization:      MetricConfig{Enabled: false},
				K8sPodCPUUsage:               MetricConfig{Enabled: true},
				K8sPodCPUUtilization:         MetricConfig{Enabled: false},
				K8sNodeCPUUsage:              MetricConfig{Enabled: true},
				K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
				K8sNodeCPUTime:               MetricConfig{Enabled: false},
				K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
				K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
				K8sNodeNetworkIO:             MetricConfig{Enabled: false},
				K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
			},
			ExtraMetadataLabels: []string{
				"k8s.volume.type",
			},
		}
		require.Equal(t, expectedKubeletStatsReceiver, *collectorConfig.Receivers.KubeletStats)
	})

	t.Run("runtime input enabled with only node metrics enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(true).
				WithRuntimeInputVolumeMetrics(false).
				Build(),
		}, BuildOptions{})

		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

		expectedKubeletStatsReceiver := KubeletStatsReceiver{
			CollectionInterval: "30s",
			AuthType:           "serviceAccount",
			Endpoint:           "https://${MY_NODE_NAME}:10250",
			InsecureSkipVerify: true,
			MetricGroups:       []MetricGroupType{"node"},
			Metrics: KubeletStatsMetricsConfig{
				ContainerCPUUsage:            MetricConfig{Enabled: true},
				ContainerCPUUtilization:      MetricConfig{Enabled: false},
				K8sPodCPUUsage:               MetricConfig{Enabled: true},
				K8sPodCPUUtilization:         MetricConfig{Enabled: false},
				K8sNodeCPUUsage:              MetricConfig{Enabled: true},
				K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
				K8sNodeCPUTime:               MetricConfig{Enabled: false},
				K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
				K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
				K8sNodeNetworkIO:             MetricConfig{Enabled: false},
				K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
			},
			ExtraMetadataLabels: []string{
				"k8s.volume.type",
			},
		}
		require.Equal(t, expectedKubeletStatsReceiver, *collectorConfig.Receivers.KubeletStats)
	})

	t.Run("runtime input enabled with only volume metrics enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(true).
				Build(),
		}, BuildOptions{})

		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

		expectedKubeletStatsReceiver := KubeletStatsReceiver{
			CollectionInterval: "30s",
			AuthType:           "serviceAccount",
			Endpoint:           "https://${MY_NODE_NAME}:10250",
			InsecureSkipVerify: true,
			MetricGroups:       []MetricGroupType{"volume"},
			Metrics: KubeletStatsMetricsConfig{
				ContainerCPUUsage:            MetricConfig{Enabled: true},
				ContainerCPUUtilization:      MetricConfig{Enabled: false},
				K8sPodCPUUsage:               MetricConfig{Enabled: true},
				K8sPodCPUUtilization:         MetricConfig{Enabled: false},
				K8sNodeCPUUsage:              MetricConfig{Enabled: true},
				K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
				K8sNodeCPUTime:               MetricConfig{Enabled: false},
				K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
				K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
				K8sNodeNetworkIO:             MetricConfig{Enabled: false},
				K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
			},
			ExtraMetadataLabels: []string{
				"k8s.volume.type",
			},
		}
		require.Equal(t, expectedKubeletStatsReceiver, *collectorConfig.Receivers.KubeletStats)
	})

	t.Run("prometheus input enabled", func(t *testing.T) {
		tests := []struct {
			name                      string
			istioEnabled              bool
			expectedPodScrapeJobs     []string
			expectedServiceScrapeJobs []string
		}{
			{
				name: "istio not enabled",
				expectedPodScrapeJobs: []string{
					"app-pods",
				},
				expectedServiceScrapeJobs: []string{
					"app-services",
				},
			},
			{
				name:         "istio enabled",
				istioEnabled: true,
				expectedPodScrapeJobs: []string{
					"app-pods",
					"app-pods-secure",
				},
				expectedServiceScrapeJobs: []string{
					"app-services",
					"app-services-secure",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
					testutils.NewMetricPipelineBuilder().WithPrometheusInput(true).Build(),
				}, BuildOptions{
					IstioEnabled: tt.istioEnabled,
				})

				receivers := collectorConfig.Receivers

				require.Nil(t, receivers.KubeletStats)
				require.Nil(t, receivers.PrometheusIstio)

				require.NotNil(t, receivers.PrometheusAppPods)
				require.Len(t, receivers.PrometheusAppPods.Config.ScrapeConfigs, len(tt.expectedPodScrapeJobs))
				for i := range receivers.PrometheusAppPods.Config.ScrapeConfigs {
					require.Equal(t, tt.expectedPodScrapeJobs[i], receivers.PrometheusAppPods.Config.ScrapeConfigs[i].JobName)
				}

				require.NotNil(t, receivers.PrometheusAppServices)
				require.Len(t, receivers.PrometheusAppServices.Config.ScrapeConfigs, len(tt.expectedServiceScrapeJobs))
				for i := range receivers.PrometheusAppServices.Config.ScrapeConfigs {
					require.Equal(t, tt.expectedServiceScrapeJobs[i], receivers.PrometheusAppServices.Config.ScrapeConfigs[i].JobName)
				}
			})
		}
	})

	t.Run("istio input enabled", func(t *testing.T) {
		collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
			testutils.NewMetricPipelineBuilder().WithIstioInput(true).Build(),
		}, BuildOptions{
			IstioEnabled: true,
		})

		require.Nil(t, collectorConfig.Receivers.KubeletStats)
		require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
		require.NotNil(t, collectorConfig.Receivers.PrometheusIstio)
		require.Len(t, collectorConfig.Receivers.PrometheusIstio.Config.ScrapeConfigs, 1)
		require.Len(t, collectorConfig.Receivers.PrometheusIstio.Config.ScrapeConfigs[0].KubernetesDiscoveryConfigs, 1)
	})
}
