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

func TestK8sClusterReceiverConfig(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewClientBuilder().Build()
	sut := Builder{
		Reader: fakeClient,
	}

	tests := []struct {
		name            string
		pipeline        telemetryv1beta1.MetricPipeline
		expectedMetrics K8sClusterMetrics
	}{
		{
			name: "default resources enabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				Build(),

			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
			},
		},
		{
			name: "only pod metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(false).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterPodMetrics: &K8sClusterPodMetrics{
					K8sPodPhase: &Metric{false},
				},
			},
		},
		{
			name: "only container metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputContainerMetrics(false).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterContainerMetrics: &K8sClusterContainerMetrics{
					K8sContainerCPURequest:    &Metric{false},
					K8sContainerCPULimit:      &Metric{false},
					K8sContainerMemoryRequest: &Metric{false},
					K8sContainerMemoryLimit:   &Metric{false},
					K8sContainerRestarts:      &Metric{false},
				},
			},
		},
		{
			name: "only statefulset metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputStatefulSetMetrics(false).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterStatefulSetMetrics: &K8sClusterStatefulSetMetrics{
					K8sStatefulSetCurrentPods: &Metric{false},
					K8sStatefulSetDesiredPods: &Metric{false},
					K8sStatefulSetReadyPods:   &Metric{false},
					K8sStatefulSetUpdatedPods: &Metric{false},
				},
			},
		}, {
			name: "only job metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputJobMetrics(false).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterJobMetrics: &K8sClusterJobMetrics{
					K8sJobActivePods:            &Metric{false},
					K8sJobDesiredSuccessfulPods: &Metric{false},
					K8sJobFailedPods:            &Metric{false},
					K8sJobMaxParallelPods:       &Metric{false},
					K8sJobSuccessfulPods:        &Metric{false},
				},
			},
		}, {
			name: "only deployment metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputDeploymentMetrics(false).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterDeploymentMetrics: &K8sClusterDeploymentMetrics{
					K8sDeploymentAvailable: &Metric{false},
					K8sDeploymentDesired:   &Metric{false},
				},
			},
		}, {
			name: "only daemonset metrics disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputDaemonSetMetrics(false).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterDaemonSetMetrics: &K8sClusterDaemonSetMetrics{
					K8sDaemonSetCurrentScheduledNodes: &Metric{false},
					K8sDaemonSetDesiredScheduledNodes: &Metric{false},
					K8sDaemonSetMisscheduledNodes:     &Metric{false},
					K8sDaemonSetReadyNodes:            &Metric{false},
				},
			},
		},
		{
			name: "Additional metrics overrule resource selectors",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputPodMetrics(false).
				WithRuntimeInputContainerMetrics(false).
				WithRuntimeInputStatefulSetMetrics(false).
				WithRuntimeInputJobMetrics(false).
				WithRuntimeInputDeploymentMetrics(false).
				WithRuntimeInputDaemonSetMetrics(false).
				WithRuntimeInputAdditionalMetrics(
					// a default pod metric
					"k8s.pod.phase",
					// a default container metric
					"k8s.container.cpu_request",
					// a default statefulset metric
					"k8s.statefulset.current_pods",
					// a default job metric
					"k8s.job.active_pods",
					// a default deployment metric
					"k8s.deployment.available",
					// a default daemonset metric
					"k8s.daemonset.current_scheduled_nodes",
				).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: getExpectedK8sClusterDefaultMetricsToDrop(),
				K8sClusterPodMetrics: &K8sClusterPodMetrics{
					K8sPodPhase: &Metric{true},
				},
				K8sClusterContainerMetrics: &K8sClusterContainerMetrics{
					K8sContainerCPURequest:    &Metric{true},
					K8sContainerCPULimit:      &Metric{false},
					K8sContainerMemoryRequest: &Metric{false},
					K8sContainerMemoryLimit:   &Metric{false},
					K8sContainerRestarts:      &Metric{false},
				},
				K8sClusterStatefulSetMetrics: &K8sClusterStatefulSetMetrics{
					K8sStatefulSetCurrentPods: &Metric{true},
					K8sStatefulSetDesiredPods: &Metric{false},
					K8sStatefulSetReadyPods:   &Metric{false},
					K8sStatefulSetUpdatedPods: &Metric{false},
				},
				K8sClusterJobMetrics: &K8sClusterJobMetrics{
					K8sJobActivePods:            &Metric{true},
					K8sJobDesiredSuccessfulPods: &Metric{false},
					K8sJobFailedPods:            &Metric{false},
					K8sJobMaxParallelPods:       &Metric{false},
					K8sJobSuccessfulPods:        &Metric{false},
				},
				K8sClusterDeploymentMetrics: &K8sClusterDeploymentMetrics{
					K8sDeploymentAvailable: &Metric{true},
					K8sDeploymentDesired:   &Metric{false},
				},
				K8sClusterDaemonSetMetrics: &K8sClusterDaemonSetMetrics{
					K8sDaemonSetCurrentScheduledNodes: &Metric{true},
					K8sDaemonSetDesiredScheduledNodes: &Metric{false},
					K8sDaemonSetMisscheduledNodes:     &Metric{false},
					K8sDaemonSetReadyNodes:            &Metric{false},
				},
			},
		},
		{
			name: "all additional metrics are provided",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics(K8sClusterReceiverMetrics...).
				Build(),
			expectedMetrics: K8sClusterMetrics{
				K8sClusterDefaultMetricsToDrop: &K8sClusterDefaultMetricsToDrop{
					K8sContainerStorageRequest:          &Metric{Enabled: true},
					K8sContainerStorageLimit:            &Metric{Enabled: true},
					K8sContainerEphemeralStorageRequest: &Metric{Enabled: true},
					K8sContainerEphemeralStorageLimit:   &Metric{Enabled: true},
					K8sContainerReady:                   &Metric{Enabled: true},
					K8sNamespacePhase:                   &Metric{Enabled: true},
					K8sHPACurrentReplicas:               &Metric{Enabled: true},
					K8sHPADesiredReplicas:               &Metric{Enabled: true},
					K8sHPAMinReplicas:                   &Metric{Enabled: true},
					K8sHPAMaxReplicas:                   &Metric{Enabled: true},
					K8sReplicaSetAvailable:              &Metric{Enabled: true},
					K8sReplicaSetDesired:                &Metric{Enabled: true},
					K8sReplicationControllerAvailable:   &Metric{Enabled: true},
					K8sReplicationControllerDesired:     &Metric{Enabled: true},
					K8sResourceQuotaHardLimit:           &Metric{Enabled: true},
					K8sResourceQuotaUsed:                &Metric{Enabled: true},
					K8sCronJobActiveJobs:                &Metric{Enabled: true},
				},
				K8sClusterPodMetrics: &K8sClusterPodMetrics{
					K8sPodPhase: &Metric{true},
				},
				K8sClusterContainerMetrics: &K8sClusterContainerMetrics{
					K8sContainerCPURequest:    &Metric{true},
					K8sContainerCPULimit:      &Metric{true},
					K8sContainerMemoryRequest: &Metric{true},
					K8sContainerMemoryLimit:   &Metric{true},
					K8sContainerRestarts:      &Metric{true},
				},
				K8sClusterStatefulSetMetrics: &K8sClusterStatefulSetMetrics{
					K8sStatefulSetCurrentPods: &Metric{true},
					K8sStatefulSetDesiredPods: &Metric{true},
					K8sStatefulSetReadyPods:   &Metric{true},
					K8sStatefulSetUpdatedPods: &Metric{true},
				},
				K8sClusterJobMetrics: &K8sClusterJobMetrics{
					K8sJobActivePods:            &Metric{true},
					K8sJobDesiredSuccessfulPods: &Metric{true},
					K8sJobFailedPods:            &Metric{true},
					K8sJobMaxParallelPods:       &Metric{true},
					K8sJobSuccessfulPods:        &Metric{true},
				},
				K8sClusterDeploymentMetrics: &K8sClusterDeploymentMetrics{
					K8sDeploymentAvailable: &Metric{true},
					K8sDeploymentDesired:   &Metric{true},
				},
				K8sClusterDaemonSetMetrics: &K8sClusterDaemonSetMetrics{
					K8sDaemonSetCurrentScheduledNodes: &Metric{true},
					K8sDaemonSetDesiredScheduledNodes: &Metric{true},
					K8sDaemonSetMisscheduledNodes:     &Metric{true},
					K8sDaemonSetReadyNodes:            &Metric{true},
				},
				K8sClusterOptionalMetrics: &K8sClusterOptionalMetrics{
					K8sContainerStatusReason:           &Metric{Enabled: true},
					K8sContainerStatusState:            &Metric{Enabled: true},
					K8sNodeCondition:                   &Metric{Enabled: true},
					K8sPodStatusReason:                 &Metric{Enabled: true},
					K8sServiceEndpointCount:            &Metric{Enabled: true},
					K8sServiceLoadBalancerIngressCount: &Metric{Enabled: true},
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

		expectedK8sClusterReceiverConfig := &K8sClusterReceiverConfig{
			AuthType:               "serviceAccount",
			CollectionInterval:     "30s",
			NodeConditionsToReport: []string{},
			K8sLeaderElector:       "k8s_leader_elector",
			Metrics:                test.expectedMetrics,
		}

		require.Contains(t, collectorConfig.Receivers, "k8s_cluster")
		require.Equal(t, expectedK8sClusterReceiverConfig, collectorConfig.Receivers["k8s_cluster"].(*K8sClusterReceiverConfig))
	}
}

func getExpectedK8sClusterDefaultMetricsToDrop() *K8sClusterDefaultMetricsToDrop {
	return &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          &Metric{Enabled: false},
		K8sContainerStorageLimit:            &Metric{Enabled: false},
		K8sContainerEphemeralStorageRequest: &Metric{Enabled: false},
		K8sContainerEphemeralStorageLimit:   &Metric{Enabled: false},
		K8sContainerReady:                   &Metric{Enabled: false},
		K8sNamespacePhase:                   &Metric{Enabled: false},
		K8sHPACurrentReplicas:               &Metric{Enabled: false},
		K8sHPADesiredReplicas:               &Metric{Enabled: false},
		K8sHPAMinReplicas:                   &Metric{Enabled: false},
		K8sHPAMaxReplicas:                   &Metric{Enabled: false},
		K8sReplicaSetAvailable:              &Metric{Enabled: false},
		K8sReplicaSetDesired:                &Metric{Enabled: false},
		K8sReplicationControllerAvailable:   &Metric{Enabled: false},
		K8sReplicationControllerDesired:     &Metric{Enabled: false},
		K8sResourceQuotaHardLimit:           &Metric{Enabled: false},
		K8sResourceQuotaUsed:                &Metric{Enabled: false},
		K8sCronJobActiveJobs:                &Metric{Enabled: false},
	}
}
