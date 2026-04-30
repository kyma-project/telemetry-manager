package metricagent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestKubeletStatsReceiverConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{
		Reader: fakeClient,
	}

	t.Run("runtime input enabled verify kubeletStatsReceiver", func(t *testing.T) {
		tests := []struct {
			name                 string
			pipeline             telemetryv1beta1.MetricPipeline
			expectedMetricGroups []MetricGroupType
		}{
			{
				name:                 "default resources enabled",
				pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
				expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
			},
			{
				name: "only pod metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypeNode, MetricGroupTypeVolume},
			},
			{
				name: "only container metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
			},
			{
				name: "only node metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputNodeMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeVolume},
			},
			{
				name: "only volume metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputVolumeMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode},
			},
		}

		for _, test := range tests {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1beta1.MetricPipeline{
				test.pipeline,
			}, BuildOptions{
				CollectionIntervals: telemetryutils.ResolveMetricCollectionIntervals(nil),
			})
			require.NoError(t, err)

			require.NotContains(t, collectorConfig.Receivers, "prometheus/app-pods")
			require.NotContains(t, collectorConfig.Receivers, "prometheus/istio")

			expectedKubeletStatsReceiverConfig := KubeletStatsReceiverConfig{
				CollectionInterval: "30s",
				AuthType:           "serviceAccount",
				Endpoint:           "https://${MY_NODE_NAME}:10250",
				InsecureSkipVerify: true,
				MetricGroups:       test.expectedMetricGroups,
				Metrics: KubeletStatsMetrics{
					ContainerCPUUsage:            Metric{Enabled: true},
					K8sPodCPUUsage:               Metric{Enabled: true},
					K8sNodeCPUUsage:              Metric{Enabled: true},
					K8sNodeCPUTime:               Metric{Enabled: false},
					K8sNodeMemoryMajorPageFaults: Metric{Enabled: false},
					K8sNodeMemoryPageFaults:      Metric{Enabled: false},
				},
				ResourceAttributes: KubeletStatsResourceAttributes{
					AWSVolumeID:            Metric{Enabled: false},
					FSType:                 Metric{Enabled: false},
					GCEPDName:              Metric{Enabled: false},
					GlusterFSEndpointsName: Metric{Enabled: false},
					GlusterFSPath:          Metric{Enabled: false},
					Partition:              Metric{Enabled: false},
				},
				ExtraMetadataLabels: []string{
					"k8s.volume.type",
				},
				CollectAllNetworkInterfaces: NetworkInterfacesEnabler{
					NodeMetrics: true,
				},
			}

			require.Contains(t, collectorConfig.Receivers, "kubeletstats")
			require.Equal(t, expectedKubeletStatsReceiverConfig, *collectorConfig.Receivers["kubeletstats"].(*KubeletStatsReceiverConfig))
		}
	})
}
