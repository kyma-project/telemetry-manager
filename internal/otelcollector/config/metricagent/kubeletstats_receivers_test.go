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

	tests := []struct {
		name                 string
		pipeline             telemetryv1beta1.MetricPipeline
		expectedMetricGroups []MetricGroupType
		expectedMetrics      KubeletStatsMetrics
	}{
		{
			name:                 "default resources enabled",
			pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
			expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
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
			expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypeNode, MetricGroupTypeVolume},
			expectedMetrics: KubeletStatsMetrics{
				KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
					K8sNodeCPUTime:               &Metric{Enabled: false},
					K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
					K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
				},
			},
		},
		{
			name: "only container metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				Build(),
			expectedMetricGroups: []MetricGroupType{MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
			expectedMetrics: KubeletStatsMetrics{
				KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
					K8sNodeCPUTime:               &Metric{Enabled: false},
					K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
					K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
				},
			},
		},
		{
			name: "only node metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputNodeMetrics(false).
				Build(),
			expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeVolume},
			expectedMetrics: KubeletStatsMetrics{
				KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
					K8sNodeCPUTime:               &Metric{Enabled: false},
					K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
					K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
				},
			},
		},
		{
			name: "only volume metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputVolumeMetrics(false).
				Build(),
			expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode},
			expectedMetrics: KubeletStatsMetrics{
				KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
					K8sNodeCPUTime:               &Metric{Enabled: false},
					K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
					K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
				},
			},
		},
		{
			name: "Additional metrics overrule resource selectors",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputNodeMetrics(false).
				WithRuntimeInputVolumeMetrics(false).
				WithRuntimeInputAdditionalMetrics(
					// default container metric
					"container.cpu.time",
					// optional container metric
					"k8s.container.cpu_limit_utilization",
					// default pod metric
					"k8s.pod.cpu.time",
					// optional pod metric
					"k8s.pod.cpu_limit_utilization",
					// default node metric
					"k8s.node.cpu.usage",
					// optional node metric
					"k8s.node.uptime",
					// default volume metric
					"k8s.volume.available",
				).
				Build(),
			expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
			expectedMetrics: KubeletStatsMetrics{
				KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
					K8sNodeCPUTime:               &Metric{Enabled: false},
					K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
					K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
				},
				KubeletStatsContainerMetrics: &KubeletStatsContainerMetrics{
					ContainerCPUTime:              &Metric{Enabled: true},
					ContainerCPUUsage:             &Metric{Enabled: false},
					ContainerFSAvailable:          &Metric{Enabled: false},
					ContainerFSCapacity:           &Metric{Enabled: false},
					ContainerFSUsage:              &Metric{Enabled: false},
					ContainerMemoryAvailable:      &Metric{Enabled: false},
					ContainerMemoryMajorPageFault: &Metric{Enabled: false},
					ContainerMemoryPageFaults:     &Metric{Enabled: false},
					ContainerMemoryRSS:            &Metric{Enabled: false},
					ContainerMemoryUsage:          &Metric{Enabled: false},
					ContainerMemoryWorkingSet:     &Metric{Enabled: false},
				},
				KubeletStatsPodMetrics: &KubeletStatsPodMetrics{
					K8sPodCPUTime:              &Metric{Enabled: true},
					K8sPodCPUUsage:             &Metric{Enabled: false},
					K8sPodFSAvailable:          &Metric{Enabled: false},
					K8sPodFSCapacity:           &Metric{Enabled: false},
					K8sPodFSUsage:              &Metric{Enabled: false},
					K8sPodMemoryAvailable:      &Metric{Enabled: false},
					K8sPodMemoryMajorPageFault: &Metric{Enabled: false},
					K8sPodMemoryPageFaults:     &Metric{Enabled: false},
					K8sPodMemoryRSS:            &Metric{Enabled: false},
					K8sPodMemoryUsage:          &Metric{Enabled: false},
					K8sPodMemoryWorkingSet:     &Metric{Enabled: false},
					K8sPodNetworkErrors:        &Metric{Enabled: false},
					K8sPodNetworkIO:            &Metric{Enabled: false},
				},
				KubeletStatsNodeMetrics: &KubeletStatsNodeMetrics{
					K8sNodeCPUUsage:         &Metric{Enabled: true},
					K8sNodeFSAvailable:      &Metric{Enabled: false},
					K8sNodeFSCapacity:       &Metric{Enabled: false},
					K8sNodeFSUsage:          &Metric{Enabled: false},
					K8sNodeMemoryAvailable:  &Metric{Enabled: false},
					K8sNodeMemoryRSS:        &Metric{Enabled: false},
					K8sNodeMemoryUsage:      &Metric{Enabled: false},
					K8sNodeMemoryWorkingSet: &Metric{Enabled: false},
					K8sNodeNetworkErrors:    &Metric{Enabled: false},
					K8sNodeNetworkIO:        &Metric{Enabled: false},
				},
				KubeletStatsVolumeMetrics: &KubeletStatsVolumeMetrics{
					K8sVolumeAvailable:  &Metric{Enabled: true},
					K8sVolumeCapacity:   &Metric{Enabled: false},
					K8sVolumeInodes:     &Metric{Enabled: false},
					K8sVolumeInodesFree: &Metric{Enabled: false},
					K8sVolumeInodesUsed: &Metric{Enabled: false},
				},
				KubeletStatsOptionalMetrics: &KubeletStatsOptionalMetrics{
					K8sContainerCPULimitUtilization: &Metric{Enabled: true},
					K8sPodCPULimitUtilization:       &Metric{Enabled: true},
					K8sNodeUptime:                   &Metric{Enabled: true},
				},
			},
		},
		{
			name: "all additional metrics are provided",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics(KubeletStatsReceiverMetrics...).
				Build(),
			expectedMetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
			expectedMetrics: KubeletStatsMetrics{
				KubeletStatsDefaultMetricsToDrop: &KubeletStatsDefaultMetricsToDrop{
					K8sNodeCPUTime:               &Metric{Enabled: true},
					K8sNodeMemoryMajorPageFaults: &Metric{Enabled: true},
					K8sNodeMemoryPageFaults:      &Metric{Enabled: true},
				},
				KubeletStatsContainerMetrics: &KubeletStatsContainerMetrics{
					ContainerCPUTime:              &Metric{Enabled: true},
					ContainerCPUUsage:             &Metric{Enabled: true},
					ContainerFSAvailable:          &Metric{Enabled: true},
					ContainerFSCapacity:           &Metric{Enabled: true},
					ContainerFSUsage:              &Metric{Enabled: true},
					ContainerMemoryAvailable:      &Metric{Enabled: true},
					ContainerMemoryMajorPageFault: &Metric{Enabled: true},
					ContainerMemoryPageFaults:     &Metric{Enabled: true},
					ContainerMemoryRSS:            &Metric{Enabled: true},
					ContainerMemoryUsage:          &Metric{Enabled: true},
					ContainerMemoryWorkingSet:     &Metric{Enabled: true},
				},
				KubeletStatsPodMetrics: &KubeletStatsPodMetrics{
					K8sPodCPUTime:              &Metric{Enabled: true},
					K8sPodCPUUsage:             &Metric{Enabled: true},
					K8sPodFSAvailable:          &Metric{Enabled: true},
					K8sPodFSCapacity:           &Metric{Enabled: true},
					K8sPodFSUsage:              &Metric{Enabled: true},
					K8sPodMemoryAvailable:      &Metric{Enabled: true},
					K8sPodMemoryMajorPageFault: &Metric{Enabled: true},
					K8sPodMemoryPageFaults:     &Metric{Enabled: true},
					K8sPodMemoryRSS:            &Metric{Enabled: true},
					K8sPodMemoryUsage:          &Metric{Enabled: true},
					K8sPodMemoryWorkingSet:     &Metric{Enabled: true},
					K8sPodNetworkErrors:        &Metric{Enabled: true},
					K8sPodNetworkIO:            &Metric{Enabled: true},
				},
				KubeletStatsNodeMetrics: &KubeletStatsNodeMetrics{
					K8sNodeCPUUsage:         &Metric{Enabled: true},
					K8sNodeFSAvailable:      &Metric{Enabled: true},
					K8sNodeFSCapacity:       &Metric{Enabled: true},
					K8sNodeFSUsage:          &Metric{Enabled: true},
					K8sNodeMemoryAvailable:  &Metric{Enabled: true},
					K8sNodeMemoryRSS:        &Metric{Enabled: true},
					K8sNodeMemoryUsage:      &Metric{Enabled: true},
					K8sNodeMemoryWorkingSet: &Metric{Enabled: true},
					K8sNodeNetworkErrors:    &Metric{Enabled: true},
					K8sNodeNetworkIO:        &Metric{Enabled: true},
				},
				KubeletStatsVolumeMetrics: &KubeletStatsVolumeMetrics{
					K8sVolumeAvailable:  &Metric{Enabled: true},
					K8sVolumeCapacity:   &Metric{Enabled: true},
					K8sVolumeInodes:     &Metric{Enabled: true},
					K8sVolumeInodesFree: &Metric{Enabled: true},
					K8sVolumeInodesUsed: &Metric{Enabled: true},
				},
				KubeletStatsOptionalMetrics: &KubeletStatsOptionalMetrics{
					ContainerUptime:                   &Metric{Enabled: true},
					K8sContainerCPUNodeUtilization:    &Metric{Enabled: true},
					K8sContainerCPULimitUtilization:   &Metric{Enabled: true},
					K8sContainerCPURequestUtilization: &Metric{Enabled: true},
					K8sContainerMemNodeUtilization:    &Metric{Enabled: true},
					K8sContainerMemLimitUtilization:   &Metric{Enabled: true},
					K8sContainerMemRequestUtilization: &Metric{Enabled: true},
					K8sNodeUptime:                     &Metric{Enabled: true},
					K8sPodCPUNodeUtilization:          &Metric{Enabled: true},
					K8sPodCPULimitUtilization:         &Metric{Enabled: true},
					K8sPodCPURequestUtilization:       &Metric{Enabled: true},
					K8sPodMemNodeUtilization:          &Metric{Enabled: true},
					K8sPodMemLimitUtilization:         &Metric{Enabled: true},
					K8sPodMemRequestUtilization:       &Metric{Enabled: true},
					K8sPodUptime:                      &Metric{Enabled: true},
					K8sPodVolumeUsage:                 &Metric{Enabled: true},
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
			MetricGroups:       test.expectedMetricGroups,
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
		}

		require.Contains(t, collectorConfig.Receivers, "kubeletstats")
		require.Equal(t, expectedKubeletStatsReceiverConfig, *collectorConfig.Receivers["kubeletstats"].(*KubeletStatsReceiverConfig))
	}

}
