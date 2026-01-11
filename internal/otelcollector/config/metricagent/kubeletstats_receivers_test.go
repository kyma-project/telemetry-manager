package metricagent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
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

func TestKubeletStatsReceiver(t *testing.T) {
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
				AgentNamespace: agentNamespace,
			})
			require.NoError(t, err)

			require.NotContains(t, collectorConfig.Receivers, "prometheus/app-pods")
			require.NotContains(t, collectorConfig.Receivers, "prometheus/istio")

			require.Contains(t, collectorConfig.Receivers, "k8s_cluster")
			k8sClusterReceiver := collectorConfig.Receivers["k8s_cluster"].(*K8sClusterReceiver)
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
			}, BuildOptions{})
			require.NoError(t, err)

			require.NotContains(t, collectorConfig.Receivers, "prometheus/app-pods")
			require.NotContains(t, collectorConfig.Receivers, "prometheus/istio")

			expectedKubeletStatsReceiver := KubeletStatsReceiver{
				CollectionInterval: "30s",
				AuthType:           "serviceAccount",
				Endpoint:           "https://${MY_NODE_NAME}:10250",
				InsecureSkipVerify: true,
				MetricGroups:       test.expectedMetricGroups,
				Metrics: KubeletStatsMetricsConfig{
					ContainerCPUUsage:            MetricConfig{Enabled: true},
					K8sPodCPUUsage:               MetricConfig{Enabled: true},
					K8sNodeCPUUsage:              MetricConfig{Enabled: true},
					K8sNodeCPUTime:               MetricConfig{Enabled: false},
					K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
					K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
				},
				ExtraMetadataLabels: []string{
					"k8s.volume.type",
				},
				CollectAllNetworkInterfaces: NetworkInterfacesEnablerConfig{
					NodeMetrics: true,
				},
			}

			require.Contains(t, collectorConfig.Receivers, "kubeletstats")
			require.Equal(t, expectedKubeletStatsReceiver, *collectorConfig.Receivers["kubeletstats"].(*KubeletStatsReceiver))
		}
	})
}

func getExpectedK8sClusterMetricsToDrop(disabledMetricResource metricResource) K8sClusterMetricsToDrop {
	metricsToDrop := K8sClusterMetricsToDrop{}

	//nolint:dupl // repeating the code as we want to test the metrics are disabled correctly
	defaultMetricsToDrop := &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          MetricConfig{Enabled: false},
		K8sContainerStorageLimit:            MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageRequest: MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageLimit:   MetricConfig{Enabled: false},
		K8sContainerReady:                   MetricConfig{Enabled: false},
		K8sNamespacePhase:                   MetricConfig{Enabled: false},
		K8sHPACurrentReplicas:               MetricConfig{Enabled: false},
		K8sHPADesiredReplicas:               MetricConfig{Enabled: false},
		K8sHPAMinReplicas:                   MetricConfig{Enabled: false},
		K8sHPAMaxReplicas:                   MetricConfig{Enabled: false},
		K8sReplicaSetAvailable:              MetricConfig{Enabled: false},
		K8sReplicaSetDesired:                MetricConfig{Enabled: false},
		K8sReplicationControllerAvailable:   MetricConfig{Enabled: false},
		K8sReplicationControllerDesired:     MetricConfig{Enabled: false},
		K8sResourceQuotaHardLimit:           MetricConfig{Enabled: false},
		K8sResourceQuotaUsed:                MetricConfig{Enabled: false},
	}
	podMetricsToDrop := &K8sClusterPodMetricsToDrop{
		K8sPodPhase: MetricConfig{false},
	}
	containerMetricsToDrop := &K8sClusterContainerMetricsToDrop{
		K8sContainerCPURequest:    MetricConfig{false},
		K8sContainerCPULimit:      MetricConfig{false},
		K8sContainerMemoryRequest: MetricConfig{false},
		K8sContainerMemoryLimit:   MetricConfig{false},
		K8sContainerRestarts:      MetricConfig{false},
	}
	statefulMetricsToDrop := &K8sClusterStatefulSetMetricsToDrop{
		K8sStatefulSetCurrentPods: MetricConfig{false},
		K8sStatefulSetDesiredPods: MetricConfig{false},
		K8sStatefulSetReadyPods:   MetricConfig{false},
		K8sStatefulSetUpdatedPods: MetricConfig{false},
	}
	jobMetricsToDrop := &K8sClusterJobMetricsToDrop{
		K8sJobActivePods:            MetricConfig{false},
		K8sJobDesiredSuccessfulPods: MetricConfig{false},
		K8sJobFailedPods:            MetricConfig{false},
		K8sJobMaxParallelPods:       MetricConfig{false},
		K8sJobSuccessfulPods:        MetricConfig{false},
	}
	deploymentMetricsToDrop := &K8sClusterDeploymentMetricsToDrop{
		K8sDeploymentAvailable: MetricConfig{false},
		K8sDeploymentDesired:   MetricConfig{false},
	}
	daemonSetMetricsToDrop := &K8sClusterDaemonSetMetricsToDrop{
		K8sDaemonSetCurrentScheduledNodes: MetricConfig{false},
		K8sDaemonSetDesiredScheduledNodes: MetricConfig{false},
		K8sDaemonSetMisscheduledNodes:     MetricConfig{false},
		K8sDaemonSetReadyNodes:            MetricConfig{false},
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
