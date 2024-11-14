package agent

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
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

	t.Run("runtime input enabled verify k8sClusterReceiver", func(t *testing.T) {
		agentNamespace := "test-namespace"

		tests := []struct {
			name                  string
			pipeline              telemetryv1alpha1.MetricPipeline
			expectedMetricsToDrop K8sClusterMetricsToDrop
		}{
			{
				name: "default resources enabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					Build(),

				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop("default"),
			},
			{
				name: "only statefulset metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputStatefulSetMetrics(true).Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop("statefulset"),
			}, {
				name: "only job metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputJobMetrics(true).Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop("job"),
			}, {
				name: "only deployment metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputDeploymentMetrics(true).Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop("deployment"),
			}, {
				name: "only daemonset metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputDaemonSetMetrics(true).Build(),
				expectedMetricsToDrop: getExpectedK8sClusterMetricsToDrop("daemonset"),
			},
		}
		for _, test := range tests {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				test.pipeline,
			}, BuildOptions{
				AgentNamespace: agentNamespace,
			})

			require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
			require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

			singletonK8sClusterReceiverCreator := collectorConfig.Receivers.SingletonK8sClusterReceiverCreator
			require.NotNil(t, singletonK8sClusterReceiverCreator)
			require.Equal(t, "serviceAccount", singletonK8sClusterReceiverCreator.AuthType)
			require.Equal(t, "telemetry-metric-agent-k8scluster", singletonK8sClusterReceiverCreator.LeaderElection.LeaseName)
			require.Equal(t, agentNamespace, singletonK8sClusterReceiverCreator.LeaderElection.LeaseNamespace)

			k8sClusterReceiver := singletonK8sClusterReceiverCreator.SingletonK8sClusterReceiver.K8sClusterReceiver
			require.Equal(t, "serviceAccount", k8sClusterReceiver.AuthType)
			require.Equal(t, "30s", k8sClusterReceiver.CollectionInterval)
			require.Len(t, k8sClusterReceiver.NodeConditionsToReport, 0)
			require.Equal(t, test.expectedMetricsToDrop, k8sClusterReceiver.Metrics)
		}
	})

	t.Run("runtime input enabled verify kubeletStatsReceiver", func(t *testing.T) {
		tests := []struct {
			name                 string
			pipeline             telemetryv1alpha1.MetricPipeline
			expectedMetricGroups []MetricGroupType
		}{
			{
				name:                 "default resources enabled",
				pipeline:             testutils.NewMetricPipelineBuilder().WithRuntimeInput(true).Build(),
				expectedMetricGroups: []MetricGroupType{"container", "pod"},
			},
			{
				name: "only pod metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputPodMetrics(true).
					WithRuntimeInputNodeMetrics(false).
					WithRuntimeInputVolumeMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{"pod"},
			},
			{
				name: "only container metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(true).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputNodeMetrics(false).
					WithRuntimeInputVolumeMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{"container"},
			},
			{
				name: "only node metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputNodeMetrics(true).
					WithRuntimeInputVolumeMetrics(false).
					Build(),
				expectedMetricGroups: []MetricGroupType{"node"},
			},
			{
				name: "only volume metrics enabled",
				pipeline: testutils.NewMetricPipelineBuilder().
					WithRuntimeInput(true).
					WithRuntimeInputContainerMetrics(false).
					WithRuntimeInputPodMetrics(false).
					WithRuntimeInputNodeMetrics(false).
					WithRuntimeInputVolumeMetrics(true).
					Build(),
				expectedMetricGroups: []MetricGroupType{"volume"},
			},
		}

		for _, test := range tests {
			collectorConfig := sut.Build([]telemetryv1alpha1.MetricPipeline{
				test.pipeline,
			}, BuildOptions{})

			require.Nil(t, collectorConfig.Receivers.PrometheusAppPods)
			require.Nil(t, collectorConfig.Receivers.PrometheusIstio)

			expectedKubeletStatsReceiver := KubeletStatsReceiver{
				CollectionInterval: "30s",
				AuthType:           "serviceAccount",
				Endpoint:           "https://${MY_NODE_NAME}:10250",
				InsecureSkipVerify: true,
				MetricGroups:       test.expectedMetricGroups,
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
		}
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

func getExpectedK8sClusterMetricsToDrop(resourceEnabled string) K8sClusterMetricsToDrop {
	metricsToDrop := K8sClusterMetricsToDrop{}

	//nolint:dupl // repeating the code as we want to test the metrics are disabled correctly
	defaultMetricsToDrop := &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          MetricConfig{Enabled: false},
		K8sContainerStorageLimit:            MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageRequest: MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageLimit:   MetricConfig{Enabled: false},
		K8sContainerRestarts:                MetricConfig{Enabled: false},
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

	if resourceEnabled == "default" {
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = statefulMetricsToDrop
		metricsToDrop.K8sClusterJobMetricsToDrop = jobMetricsToDrop
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = deploymentMetricsToDrop
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = daemonSetMetricsToDrop
	}

	if resourceEnabled == "statefulset" {
		metricsToDrop.K8sClusterPodMetricsToDrop = podMetricsToDrop
		metricsToDrop.K8sClusterContainerMetricsToDrop = containerMetricsToDrop
		metricsToDrop.K8sClusterJobMetricsToDrop = jobMetricsToDrop
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = deploymentMetricsToDrop
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = daemonSetMetricsToDrop
	}

	if resourceEnabled == "job" {
		metricsToDrop.K8sClusterPodMetricsToDrop = podMetricsToDrop
		metricsToDrop.K8sClusterContainerMetricsToDrop = containerMetricsToDrop
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = statefulMetricsToDrop
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = deploymentMetricsToDrop
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = daemonSetMetricsToDrop
	}

	if resourceEnabled == "deployment" {
		metricsToDrop.K8sClusterPodMetricsToDrop = podMetricsToDrop
		metricsToDrop.K8sClusterContainerMetricsToDrop = containerMetricsToDrop
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = statefulMetricsToDrop
		metricsToDrop.K8sClusterJobMetricsToDrop = jobMetricsToDrop
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = daemonSetMetricsToDrop
	}

	if resourceEnabled == "daemonset" {
		metricsToDrop.K8sClusterPodMetricsToDrop = podMetricsToDrop
		metricsToDrop.K8sClusterContainerMetricsToDrop = containerMetricsToDrop
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = statefulMetricsToDrop
		metricsToDrop.K8sClusterJobMetricsToDrop = jobMetricsToDrop
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = deploymentMetricsToDrop
	}

	return metricsToDrop
}
