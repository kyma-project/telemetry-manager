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
			name            string
			pipeline        telemetryv1beta1.MetricPipeline
			expectedMetrics KubeletStatsMetrics
		}{
			{
				name:     "default resources enabled",
				pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
				expectedMetrics: KubeletStatsMetrics{
					KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
						K8sNodeCPUTime:               &Metric{Enabled: false},
						K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
						K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
					},
				},
			},
			{
				name: "only pod metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					Build(),
				expectedMetrics: KubeletStatsMetrics{
					KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
						K8sNodeCPUTime:               &Metric{Enabled: false},
						K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
						K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
					},
					KubeletStatsPodMetrics: &KubeletStatsPodMetrics{
						K8sPodCPUTime:              &Metric{false},
						K8sPodCPUUsage:             &Metric{false},
						K8sPodFSAvailable:          &Metric{false},
						K8sPodFSCapacity:           &Metric{false},
						K8sPodFSUsage:              &Metric{false},
						K8sPodMemoryAvailable:      &Metric{false},
						K8sPodMemoryMajorPageFault: &Metric{false},
						K8sPodMemoryPageFaults:     &Metric{false},
						K8sPodMemoryRSS:            &Metric{false},
						K8sPodMemoryUsage:          &Metric{false},
						K8sPodMemoryWorkingSet:     &Metric{false},
						K8sPodNetworkErrors:        &Metric{false},
						K8sPodNetworkIO:            &Metric{false},
					},
				},
			},
			{
				name: "only container metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					Build(),
				expectedMetrics: KubeletStatsMetrics{
					KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
						K8sNodeCPUTime:               &Metric{Enabled: false},
						K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
						K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
					},
					KubeletStatsContainerMetrics: &KubeletStatsContainerMetrics{
						ContainerCPUTime:              &Metric{false},
						ContainerCPUUsage:             &Metric{false},
						ContainerFSAvailable:          &Metric{false},
						ContainerFSCapacity:           &Metric{false},
						ContainerFSUsage:              &Metric{false},
						ContainerMemoryAvailable:      &Metric{false},
						ContainerMemoryMajorPageFault: &Metric{false},
						ContainerMemoryPageFaults:     &Metric{false},
						ContainerMemoryRSS:            &Metric{false},
						ContainerMemoryUsage:          &Metric{false},
						ContainerMemoryWorkingSet:     &Metric{false},
					},
				},
			},
			{
				name: "only node metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputNodeMetrics(false).
					Build(),
				expectedMetrics: KubeletStatsMetrics{
					KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
						K8sNodeCPUTime:               &Metric{Enabled: false},
						K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
						K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
					},
					KubeletStatsNodeMetrics: &KubeletStatsNodeMetrics{
						K8sNodeCPUUsage:         &Metric{false},
						K8sNodeFSAvailable:      &Metric{false},
						K8sNodeFSCapacity:       &Metric{false},
						K8sNodeFSUsage:          &Metric{false},
						K8sNodeMemoryAvailable:  &Metric{false},
						K8sNodeMemoryRSS:        &Metric{false},
						K8sNodeMemoryUsage:      &Metric{false},
						K8sNodeMemoryWorkingSet: &Metric{false},
						K8sNodeNetworkErrors:    &Metric{false},
						K8sNodeNetworkIO:        &Metric{false},
					},
				},
			},
			{
				name: "only volume metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputVolumeMetrics(false).
					Build(),
				expectedMetrics: KubeletStatsMetrics{
					KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
						K8sNodeCPUTime:               &Metric{Enabled: false},
						K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
						K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
					},
					KubeletStatsVolumeMetrics: &KubeletStatsVolumeMetrics{
						K8sVolumeAvailable:  &Metric{false},
						K8sVolumeCapacity:   &Metric{false},
						K8sVolumeInodes:     &Metric{false},
						K8sVolumeInodesFree: &Metric{false},
						K8sVolumeInodesUsed: &Metric{false},
					},
				},
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
				MetricGroups:       []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
				Metrics:            test.expectedMetrics,
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
				Node: "${MY_NODE_NAME}",
				K8sApiConfig: K8sAPIConfig{
					AuthType: "serviceAccount",
				},
			}

			require.Contains(t, collectorConfig.Receivers, "kubeletstats")
			require.Equal(t, expectedKubeletStatsReceiverConfig, *collectorConfig.Receivers["kubeletstats"].(*KubeletStatsReceiverConfig))
		}
	})
}
