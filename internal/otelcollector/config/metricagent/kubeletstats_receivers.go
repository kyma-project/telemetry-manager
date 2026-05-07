package metricagent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func kubeletStatsReceiver(runtimeResources runtimeResourceSources, additionalMetrics []string, collectionInterval time.Duration) *KubeletStatsReceiverConfig {
	const portKubelet = 10250

	return &KubeletStatsReceiverConfig{
		CollectionInterval: collectionInterval.String(),
		AuthType:           "serviceAccount",
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${%s}:%d", common.EnvVarCurrentNodeName, portKubelet),
		// include all metrics groups, then enable/disable individual metrics based on resource selectors and additional metrics.
		MetricGroups: []MetricGroupType{MetricGroupTypeNode, MetricGroupTypePod, MetricGroupTypeContainer, MetricGroupTypeVolume},
		Metrics:      kubeletStatsMetrics(runtimeResources, additionalMetrics),
		// These resource attributes have been deprecated by OTel and will be removed in future versions.
		// The volume types associated with them have already been removed for the K8S versions that we use (v1.28+).
		// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/45896
		ResourceAttributes: KubeletStatsResourceAttributes{
			AWSVolumeID:            Metric{Enabled: false},
			FSType:                 Metric{Enabled: false},
			GCEPDName:              Metric{Enabled: false},
			GlusterFSEndpointsName: Metric{Enabled: false},
			GlusterFSPath:          Metric{Enabled: false},
			Partition:              Metric{Enabled: false},
		},
		ExtraMetadataLabels: []string{"k8s.volume.type"},
		CollectAllNetworkInterfaces: NetworkInterfacesEnabler{
			NodeMetrics: true,
		},
	}
}

func kubeletStatsMetrics(runtimeResources runtimeResourceSources, additionalMetrics []string) KubeletStatsMetrics {
	metrics := KubeletStatsMetrics{}

	disableKubeletStatsMetrics(&metrics, runtimeResources)
	enableKubeletStatsAdditionalMetrics(&metrics, additionalMetrics)

	return metrics
}

func disableKubeletStatsMetrics(metrics *KubeletStatsMetrics, runtimeResources runtimeResourceSources) {
	metrics.KubeletStatsDefaultMetricsToDrop = &KubeletStatsDefaultMetricsToDrop{
		K8sNodeCPUTime:               Metric{Enabled: false},
		K8sNodeMemoryMajorPageFaults: Metric{Enabled: false},
		K8sNodeMemoryPageFaults:      Metric{Enabled: false},
	}

	// The following metrics are enabled by default in the KubeletStatsReceiver.
	// If the resource selectors are disabled, we need to disable the corresponding metrics in the KubeletStatsReceiver.

	if !runtimeResources.pod {
		metrics.KubeletStatsPodMetrics = &KubeletStatsPodMetrics{
			K8sPodCPUTime:              Metric{false},
			K8sPodCPUUsage:             Metric{false},
			K8sPodFSAvailable:          Metric{false},
			K8sPodFSCapacity:           Metric{false},
			K8sPodFSUsage:              Metric{false},
			K8sPodMemoryAvailable:      Metric{false},
			K8sPodMemoryMajorPageFault: Metric{false},
			K8sPodMemoryPageFaults:     Metric{false},
			K8sPodMemoryRSS:            Metric{false},
			K8sPodMemoryUsage:          Metric{false},
			K8sPodMemoryWorkingSet:     Metric{false},
			K8sPodNetworkErrors:        Metric{false},
			K8sPodNetworkIO:            Metric{false},
		}
	}

	if !runtimeResources.container {
		metrics.KubeletStatsContainerMetrics = &KubeletStatsContainerMetrics{
			ContainerCPUTime:              Metric{false},
			ContainerCPUUsage:             Metric{false},
			ContainerFSAvailable:          Metric{false},
			ContainerFSCapacity:           Metric{false},
			ContainerFSUsage:              Metric{false},
			ContainerMemoryAvailable:      Metric{false},
			ContainerMemoryMajorPageFault: Metric{false},
			ContainerMemoryPageFaults:     Metric{false},
			ContainerMemoryRSS:            Metric{false},
			ContainerMemoryUsage:          Metric{false},
			ContainerMemoryWorkingSet:     Metric{false},
		}
	}

	if !runtimeResources.node {
		metrics.KubeletStatsNodeMetrics = &KubeletStatsNodeMetrics{
			K8sNodeCPUUsage:         Metric{false},
			K8sNodeFSAvailable:      Metric{false},
			K8sNodeFSCapacity:       Metric{false},
			K8sNodeFSUsage:          Metric{false},
			K8sNodeMemoryAvailable:  Metric{false},
			K8sNodeMemoryRSS:        Metric{false},
			K8sNodeMemoryUsage:      Metric{false},
			K8sNodeMemoryWorkingSet: Metric{false},
			K8sNodeNetworkErrors:    Metric{false},
			K8sNodeNetworkIO:        Metric{false},
		}
	}

	if !runtimeResources.volume {
		metrics.KubeletStatsVolumeMetrics = &KubeletStatsVolumeMetrics{
			K8sVolumeAvailable:  Metric{false},
			K8sVolumeCapacity:   Metric{false},
			K8sVolumeInodes:     Metric{false},
			K8sVolumeInodesFree: Metric{false},
			K8sVolumeInodesUsed: Metric{false},
		}
	}
}

func enableKubeletStatsAdditionalMetrics(metrics *KubeletStatsMetrics, additionalMetrics []string) {
	for _, m := range additionalMetrics {
		switch m {
		// KubeletStatsDefaultMetricsToDrop
		case metricK8sNodeCPUTime:
			metrics.K8sNodeCPUTime.Enabled = true
		case metricK8sNodeMemoryMajorPageFaults:
			metrics.K8sNodeMemoryMajorPageFaults.Enabled = true
		case metricK8sNodeMemoryPageFaults:
			metrics.K8sNodeMemoryPageFaults.Enabled = true

		// KubeletStatsPodMetrics
		case metricK8sPodCPUTime:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodCPUTime.Enabled = true
		case metricK8sPodCPUUsage:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodCPUUsage.Enabled = true
		case metricK8sPodFSAvailable:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodFSAvailable.Enabled = true
		case metricK8sPodFSCapacity:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodFSCapacity.Enabled = true
		case metricK8sPodFSUsage:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodFSUsage.Enabled = true
		case metricK8sPodMemoryAvailable:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodMemoryAvailable.Enabled = true
		case metricK8sPodMemoryMajorPageFaults:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodMemoryMajorPageFault.Enabled = true
		case metricK8sPodMemoryPageFaults:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodMemoryPageFaults.Enabled = true
		case metricK8sPodMemoryRSS:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodMemoryRSS.Enabled = true
		case metricK8sPodMemoryUsage:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodMemoryUsage.Enabled = true
		case metricK8sPodMemoryWorkingSet:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodMemoryWorkingSet.Enabled = true
		case metricK8sPodNetworkErrors:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodNetworkErrors.Enabled = true
		case metricK8sPodNetworkIO:
			initKubeletStatsPodMetrics(metrics)
			metrics.K8sPodNetworkIO.Enabled = true

		// KubeletStatsContainerMetrics
		case metricContainerCPUTime:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerCPUTime.Enabled = true
		case metricContainerCPUUsage:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerCPUUsage.Enabled = true
		case metricContainerFSAvailable:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerFSAvailable.Enabled = true
		case metricContainerFSCapacity:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerFSCapacity.Enabled = true
		case metricContainerFSUsage:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerFSUsage.Enabled = true
		case metricContainerMemoryAvailable:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerMemoryAvailable.Enabled = true
		case metricContainerMemoryMajorPageFault:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerMemoryMajorPageFault.Enabled = true
		case metricContainerMemoryPageFaults:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerMemoryPageFaults.Enabled = true
		case metricContainerMemoryRSS:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerMemoryRSS.Enabled = true
		case metricContainerMemoryUsage:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerMemoryUsage.Enabled = true
		case metricContainerMemoryWorkingSet:
			initKubeletStatsContainerMetrics(metrics)
			metrics.ContainerMemoryWorkingSet.Enabled = true

		// KubeletStatsNodeMetrics
		case metricK8sNodeCPUUsage:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeCPUUsage.Enabled = true
		case metricK8sNodeFSAvailable:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeFSAvailable.Enabled = true
		case metricK8sNodeFSCapacity:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeFSCapacity.Enabled = true
		case metricK8sNodeFSUsage:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeFSUsage.Enabled = true
		case metricK8sNodeMemoryAvailable:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeMemoryAvailable.Enabled = true
		case metricK8sNodeMemoryRSS:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeMemoryRSS.Enabled = true
		case metricK8sNodeMemoryUsage:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeMemoryUsage.Enabled = true
		case metricK8sNodeMemoryWorkingSet:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeMemoryWorkingSet.Enabled = true
		case metricK8sNodeNetworkErrors:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeNetworkErrors.Enabled = true
		case metricK8sNodeNetworkIO:
			initKubeletStatsNodeMetrics(metrics)
			metrics.K8sNodeNetworkIO.Enabled = true

		// KubeletStatsVolumeMetrics
		case metricK8sVolumeAvailable:
			initKubeletStatsVolumeMetrics(metrics)
			metrics.K8sVolumeAvailable.Enabled = true
		case metricK8sVolumeCapacity:
			initKubeletStatsVolumeMetrics(metrics)
			metrics.K8sVolumeCapacity.Enabled = true
		case metricK8sVolumeInodes:
			initKubeletStatsVolumeMetrics(metrics)
			metrics.K8sVolumeInodes.Enabled = true
		case metricK8sVolumeInodesFree:
			initKubeletStatsVolumeMetrics(metrics)
			metrics.K8sVolumeInodesFree.Enabled = true
		case metricK8sVolumeInodesUsed:
			initKubeletStatsVolumeMetrics(metrics)
			metrics.K8sVolumeInodesUsed.Enabled = true

		// KubeletStatsOptionalMetrics
		case metricContainerUptime:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.ContainerUptime.Enabled = true
		case metricK8sContainerCPUNodeUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sContainerCPUNodeUtilization.Enabled = true
		case metricK8sContainerCPULimitUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sContainerCPULimitUtilization.Enabled = true
		case metricK8sContainerCPURequestUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sContainerCPURequestUtilization.Enabled = true
		case metricK8sContainerMemNodeUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sContainerMemNodeUtilization.Enabled = true
		case metricK8sContainerMemLimitUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sContainerMemLimitUtilization.Enabled = true
		case metricK8sContainerMemRequestUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sContainerMemRequestUtilization.Enabled = true
		case metricK8sNodeUptime:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sNodeUptime.Enabled = true
		case metricK8sPodCPUNodeUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodCPUNodeUtilization.Enabled = true
		case metricK8sPodCPULimitUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodCPULimitUtilization.Enabled = true
		case metricK8sPodCPURequestUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodCPURequestUtilization.Enabled = true
		case metricK8sPodMemNodeUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodMemNodeUtilization.Enabled = true
		case metricK8sPodMemLimitUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodMemLimitUtilization.Enabled = true
		case metricK8sPodMemRequestUtilization:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodMemRequestUtilization.Enabled = true
		case metricK8sPodUptime:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodUptime.Enabled = true
		case metricK8sPodVolumeUsage:
			initKubeletStatsOptionalMetrics(metrics)
			metrics.K8sPodVolumeUsage.Enabled = true
		}
	}
}

func initKubeletStatsPodMetrics(metrics *KubeletStatsMetrics) {
	if metrics.KubeletStatsPodMetrics == nil {
		metrics.KubeletStatsPodMetrics = &KubeletStatsPodMetrics{}
	}
}

func initKubeletStatsContainerMetrics(metrics *KubeletStatsMetrics) {
	if metrics.KubeletStatsContainerMetrics == nil {
		metrics.KubeletStatsContainerMetrics = &KubeletStatsContainerMetrics{}
	}
}

func initKubeletStatsNodeMetrics(metrics *KubeletStatsMetrics) {
	if metrics.KubeletStatsNodeMetrics == nil {
		metrics.KubeletStatsNodeMetrics = &KubeletStatsNodeMetrics{}
	}
}

func initKubeletStatsVolumeMetrics(metrics *KubeletStatsMetrics) {
	if metrics.KubeletStatsVolumeMetrics == nil {
		metrics.KubeletStatsVolumeMetrics = &KubeletStatsVolumeMetrics{}
	}
}

func initKubeletStatsOptionalMetrics(metrics *KubeletStatsMetrics) {
	if metrics.KubeletStatsOptionalMetrics == nil {
		metrics.KubeletStatsOptionalMetrics = &KubeletStatsOptionalMetrics{}
	}
}
