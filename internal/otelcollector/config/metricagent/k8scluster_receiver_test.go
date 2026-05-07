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

func TestK8sClusterReceiverConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{
		Reader: fakeClient,
	}

	agentNamespace := "test-namespace"

	tests := []struct {
		name                  string
		pipeline              telemetryv1beta1.MetricPipeline
		expectedMetricsToDrop K8sClusterMetrics
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
}

func getExpectedK8sClusterMetricsToDrop(disabledMetricResource metricResource) K8sClusterMetrics {
	metricsToDrop := K8sClusterMetrics{}

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
	podMetricsToDrop := &K8sClusterPodMetrics{
		K8sPodPhase: Metric{false},
	}
	containerMetricsToDrop := &K8sClusterContainerMetrics{
		K8sContainerCPURequest:    Metric{false},
		K8sContainerCPULimit:      Metric{false},
		K8sContainerMemoryRequest: Metric{false},
		K8sContainerMemoryLimit:   Metric{false},
		K8sContainerRestarts:      Metric{false},
	}
	statefulMetricsToDrop := &K8sClusterStatefulSetMetrics{
		K8sStatefulSetCurrentPods: Metric{false},
		K8sStatefulSetDesiredPods: Metric{false},
		K8sStatefulSetReadyPods:   Metric{false},
		K8sStatefulSetUpdatedPods: Metric{false},
	}
	jobMetricsToDrop := &K8sClusterJobMetrics{
		K8sJobActivePods:            Metric{false},
		K8sJobDesiredSuccessfulPods: Metric{false},
		K8sJobFailedPods:            Metric{false},
		K8sJobMaxParallelPods:       Metric{false},
		K8sJobSuccessfulPods:        Metric{false},
	}
	deploymentMetricsToDrop := &K8sClusterDeploymentMetrics{
		K8sDeploymentAvailable: Metric{false},
		K8sDeploymentDesired:   Metric{false},
	}
	daemonSetMetricsToDrop := &K8sClusterDaemonSetMetrics{
		K8sDaemonSetCurrentScheduledNodes: Metric{false},
		K8sDaemonSetDesiredScheduledNodes: Metric{false},
		K8sDaemonSetMisscheduledNodes:     Metric{false},
		K8sDaemonSetReadyNodes:            Metric{false},
	}

	metricsToDrop.K8sClusterDefaultMetricsToDrop = defaultMetricsToDrop

	if disabledMetricResource == pod {
		metricsToDrop.K8sClusterPodMetrics = podMetricsToDrop
	}

	if disabledMetricResource == container {
		metricsToDrop.K8sClusterContainerMetrics = containerMetricsToDrop
	}

	if disabledMetricResource == statefulset {
		metricsToDrop.K8sClusterStatefulSetMetrics = statefulMetricsToDrop
	}

	if disabledMetricResource == job {
		metricsToDrop.K8sClusterJobMetrics = jobMetricsToDrop
	}

	if disabledMetricResource == deployment {
		metricsToDrop.K8sClusterDeploymentMetrics = deploymentMetricsToDrop
	}

	if disabledMetricResource == daemonset {
		metricsToDrop.K8sClusterDaemonSetMetrics = daemonSetMetricsToDrop
	}

	return metricsToDrop
}
