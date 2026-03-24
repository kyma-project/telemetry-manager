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

type metricResource string

const (
	pod         metricResource = "pod"
	container   metricResource = "container"
	statefulset metricResource = "statefulset"
	job         metricResource = "job"
	deployment  metricResource = "deployment"
	daemonset   metricResource = "daemonset"
	none        metricResource = "none"
)

func TestKubeletStatsReceiverConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{
		Reader: fakeClient,
	}

	t.Run("runtime input enabled verify k8sClusterReceiver", func(t *testing.T) {
		agentNamespace := "test-namespace"

		tests := []struct {
			name                  string
			pipeline              telemetryv1beta1.MetricPipeline
			expectedMetricsToDrop K8sClusterMetricsToDrop
		}{
			{
				name: "default resources enabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					Build(),

				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(none),
			},
			{
				name: "only pod metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(pod),
			},
			{
				name: "only container metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(container),
			},
			{
				name: "only statefulset metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputStatefulSetMetrics(false).
					Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(statefulset),
			}, {
				name: "only job metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputJobMetrics(false).
					Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(job),
			}, {
				name: "only deployment metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputDeploymentMetrics(false).
					Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(deployment),
			}, {
				name: "only daemonset metrics disabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputDaemonSetMetrics(false).
					Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop(daemonset),
			},
		}
		for _, test := range tests {
			collectorConfig, _, err := sut.Build(ctx, []telemetryv1beta1.MetricPipeline{
				test.pipeline,
			}, BuildOptions{
				AgentNamespace:      agentNamespace,
				CollectionIntervals: telemetryutils.ResolveMetricCollectionIntervals(nil),
			})
			require.NoError(t, err)

			require.NotContains(t, collectorConfig.Receivers, "prometheus/app-pods")
			require.NotContains(t, collectorConfig.Receivers, "prometheus/istio")

			require.Contains(t, collectorConfig.Receivers, "k8s_cluster")
			k8sClusterReceiver := collectorConfig.Receivers["k8s_cluster"].(*K8sClusterReceiverConfig)
			require.Equal(t, "serviceAccount", k8sClusterReceiver.AuthType)
			require.Equal(t, "30s", k8sClusterReceiver.CollectionInterval)
			require.Len(t, k8sClusterReceiver.NodeConditionsToReport, 0)
			require.Equal(t, test.expectedMetricsToDrop, k8sClusterReceiver.Metrics)
		}
	})

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

func getExpectedK8sClusterMetricsToDrop(disabledMetricResource metricResource) K8sClusterMetricsToDrop {
	metricsToDrop := K8sClusterMetricsToDrop{}

	//nolint:dupl // repeating the code as we want to test the metrics are disabled correctly
	defaultMetricsToDrop := &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          Metric{Enabled: false},
		K8sContainerStorageLimit:            Metric{Enabled: false},
		K8sContainerEphemeralStorageRequest: Metric{Enabled: false},
		K8sContainerEphemeralStorageLimit:   Metric{Enabled: false},
		K8sContainerReady:                   Metric{Enabled: false},
		K8sNamespacePhase:                   Metric{Enabled: false},
		K8sHPACurrentReplicas:               Metric{Enabled: false},
		K8sHPADesiredReplicas:               Metric{Enabled: false},
		K8sHPAMinReplicas:                   Metric{Enabled: false},
		K8sHPAMaxReplicas:                   Metric{Enabled: false},
		K8sReplicaSetAvailable:              Metric{Enabled: false},
		K8sReplicaSetDesired:                Metric{Enabled: false},
		K8sReplicationControllerAvailable:   Metric{Enabled: false},
		K8sReplicationControllerDesired:     Metric{Enabled: false},
		K8sResourceQuotaHardLimit:           Metric{Enabled: false},
		K8sResourceQuotaUsed:                Metric{Enabled: false},
	}
	podMetricsToDrop := &K8sClusterPodMetricsToDrop{
		K8sPodPhase: Metric{false},
	}
	containerMetricsToDrop := &K8sClusterContainerMetricsToDrop{
		K8sContainerCPURequest:    Metric{false},
		K8sContainerCPULimit:      Metric{false},
		K8sContainerMemoryRequest: Metric{false},
		K8sContainerMemoryLimit:   Metric{false},
		K8sContainerRestarts:      Metric{false},
	}
	statefulMetricsToDrop := &K8sClusterStatefulSetMetricsToDrop{
		K8sStatefulSetCurrentPods: Metric{false},
		K8sStatefulSetDesiredPods: Metric{false},
		K8sStatefulSetReadyPods:   Metric{false},
		K8sStatefulSetUpdatedPods: Metric{false},
	}
	jobMetricsToDrop := &K8sClusterJobMetricsToDrop{
		K8sJobActivePods:            Metric{false},
		K8sJobDesiredSuccessfulPods: Metric{false},
		K8sJobFailedPods:            Metric{false},
		K8sJobMaxParallelPods:       Metric{false},
		K8sJobSuccessfulPods:        Metric{false},
	}
	deploymentMetricsToDrop := &K8sClusterDeploymentMetricsToDrop{
		K8sDeploymentAvailable: Metric{false},
		K8sDeploymentDesired:   Metric{false},
	}
	daemonSetMetricsToDrop := &K8sClusterDaemonSetMetricsToDrop{
		K8sDaemonSetCurrentScheduledNodes: Metric{false},
		K8sDaemonSetDesiredScheduledNodes: Metric{false},
		K8sDaemonSetMisscheduledNodes:     Metric{false},
		K8sDaemonSetReadyNodes:            Metric{false},
	}

	metricsToDrop.K8sClusterDefaultMetricsToDrop = defaultMetricsToDrop

	if disabledMetricResource == pod {
		metricsToDrop.K8sClusterPodMetricsToDrop = podMetricsToDrop
	}

	if disabledMetricResource == container {
		metricsToDrop.K8sClusterContainerMetricsToDrop = containerMetricsToDrop
	}

	if disabledMetricResource == statefulset {
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = statefulMetricsToDrop
	}

	if disabledMetricResource == job {
		metricsToDrop.K8sClusterJobMetricsToDrop = jobMetricsToDrop
	}

	if disabledMetricResource == deployment {
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = deploymentMetricsToDrop
	}

	if disabledMetricResource == daemonset {
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = daemonSetMetricsToDrop
	}

	return metricsToDrop
}
