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
		MetricGroups: []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode, MetricGroupTypeVolume},
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
		// In order to collect k8s.{container,pod}.{cpu,memory}.node.utilization metrics, the "node" and the "k8s_api_config" fields must be set
		// For more details, check https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/kubeletstatsreceiver#collect-k8scontainerpodcpumemorynodeutilization-as-ratio-of-total-nodes-capacity
		Node: fmt.Sprintf("${%s}", common.EnvVarCurrentNodeName),
		K8sApiConfig: K8sAPIConfig{
			AuthType: "serviceAccount",
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
		K8sNodeCPUTime:               &Metric{Enabled: false},
		K8sNodeMemoryMajorPageFaults: &Metric{Enabled: false},
		K8sNodeMemoryPageFaults:      &Metric{Enabled: false},
	}

	// The following metrics are enabled by default in the KubeletStatsReceiver.
	// If the resource selectors are disabled, we need to disable the corresponding metrics in the KubeletStatsReceiver.

	if !runtimeResources.pod {
		metrics.KubeletStatsPodMetrics = &KubeletStatsPodMetrics{
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
		}
	}

	if !runtimeResources.container {
		metrics.KubeletStatsContainerMetrics = &KubeletStatsContainerMetrics{
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
		}
	}

	if !runtimeResources.node {
		metrics.KubeletStatsNodeMetrics = &KubeletStatsNodeMetrics{
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
		}
	}

	if !runtimeResources.volume {
		metrics.KubeletStatsVolumeMetrics = &KubeletStatsVolumeMetrics{
			K8sVolumeAvailable:  &Metric{false},
			K8sVolumeCapacity:   &Metric{false},
			K8sVolumeInodes:     &Metric{false},
			K8sVolumeInodesFree: &Metric{false},
			K8sVolumeInodesUsed: &Metric{false},
		}
	}
}

func enableKubeletStatsAdditionalMetrics(metrics *KubeletStatsMetrics, additionalMetrics []string) {
	for _, m := range additionalMetrics {
		if enabler, ok := kubeletStatsMetricEnablers[m]; ok {
			enabler(metrics)
		}
	}
}

var kubeletStatsMetricEnablers = map[string]func(*KubeletStatsMetrics){
	// KubeletStatsDefaultMetricsToDrop
	metricK8sNodeCPUTime: func(m *KubeletStatsMetrics) {
		m.K8sNodeCPUTime = &Metric{Enabled: true}
	},
	metricK8sNodeMemoryMajorPageFaults: func(m *KubeletStatsMetrics) {
		m.K8sNodeMemoryMajorPageFaults = &Metric{Enabled: true}
	},
	metricK8sNodeMemoryPageFaults: func(m *KubeletStatsMetrics) {
		m.K8sNodeMemoryPageFaults = &Metric{Enabled: true}
	},

	// KubeletStatsPodMetrics
	metricK8sPodCPUTime: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodCPUTime = &Metric{Enabled: true}
	},
	metricK8sPodCPUUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodCPUUsage = &Metric{Enabled: true}
	},
	metricK8sPodFSAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodFSAvailable = &Metric{Enabled: true}
	},
	metricK8sPodFSCapacity: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodFSCapacity = &Metric{Enabled: true}
	},
	metricK8sPodFSUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodFSUsage = &Metric{Enabled: true}
	},
	metricK8sPodMemoryAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodMemoryAvailable = &Metric{Enabled: true}
	},
	metricK8sPodMemoryMajorPageFaults: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodMemoryMajorPageFault = &Metric{Enabled: true}
	},
	metricK8sPodMemoryPageFaults: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodMemoryPageFaults = &Metric{Enabled: true}
	},
	metricK8sPodMemoryRSS: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodMemoryRSS = &Metric{Enabled: true}
	},
	metricK8sPodMemoryUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodMemoryUsage = &Metric{Enabled: true}
	},
	metricK8sPodMemoryWorkingSet: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodMemoryWorkingSet = &Metric{Enabled: true}
	},
	metricK8sPodNetworkErrors: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodNetworkErrors = &Metric{Enabled: true}
	},
	metricK8sPodNetworkIO: func(m *KubeletStatsMetrics) {
		initKubeletStatsPodMetrics(m)
		m.K8sPodNetworkIO = &Metric{Enabled: true}
	},

	// KubeletStatsContainerMetrics
	metricContainerCPUTime: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerCPUTime = &Metric{Enabled: true}
	},
	metricContainerCPUUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerCPUUsage = &Metric{Enabled: true}
	},
	metricContainerFSAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerFSAvailable = &Metric{Enabled: true}
	},
	metricContainerFSCapacity: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerFSCapacity = &Metric{Enabled: true}
	},
	metricContainerFSUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerFSUsage = &Metric{Enabled: true}
	},
	metricContainerMemoryAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerMemoryAvailable = &Metric{Enabled: true}
	},
	metricContainerMemoryMajorPageFault: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerMemoryMajorPageFault = &Metric{Enabled: true}
	},
	metricContainerMemoryPageFaults: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerMemoryPageFaults = &Metric{Enabled: true}
	},
	metricContainerMemoryRSS: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerMemoryRSS = &Metric{Enabled: true}
	},
	metricContainerMemoryUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerMemoryUsage = &Metric{Enabled: true}
	},
	metricContainerMemoryWorkingSet: func(m *KubeletStatsMetrics) {
		initKubeletStatsContainerMetrics(m)
		m.ContainerMemoryWorkingSet = &Metric{Enabled: true}
	},

	// KubeletStatsNodeMetrics
	metricK8sNodeCPUUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeCPUUsage = &Metric{Enabled: true}
	},
	metricK8sNodeFSAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeFSAvailable = &Metric{Enabled: true}
	},
	metricK8sNodeFSCapacity: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeFSCapacity = &Metric{Enabled: true}
	},
	metricK8sNodeFSUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeFSUsage = &Metric{Enabled: true}
	},
	metricK8sNodeMemoryAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeMemoryAvailable = &Metric{Enabled: true}
	},
	metricK8sNodeMemoryRSS: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeMemoryRSS = &Metric{Enabled: true}
	},
	metricK8sNodeMemoryUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeMemoryUsage = &Metric{Enabled: true}
	},
	metricK8sNodeMemoryWorkingSet: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeMemoryWorkingSet = &Metric{Enabled: true}
	},
	metricK8sNodeNetworkErrors: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeNetworkErrors = &Metric{Enabled: true}
	},
	metricK8sNodeNetworkIO: func(m *KubeletStatsMetrics) {
		initKubeletStatsNodeMetrics(m)
		m.K8sNodeNetworkIO = &Metric{Enabled: true}
	},

	// KubeletStatsVolumeMetrics
	metricK8sVolumeAvailable: func(m *KubeletStatsMetrics) {
		initKubeletStatsVolumeMetrics(m)
		m.K8sVolumeAvailable = &Metric{Enabled: true}
	},
	metricK8sVolumeCapacity: func(m *KubeletStatsMetrics) {
		initKubeletStatsVolumeMetrics(m)
		m.K8sVolumeCapacity = &Metric{Enabled: true}
	},
	metricK8sVolumeInodes: func(m *KubeletStatsMetrics) {
		initKubeletStatsVolumeMetrics(m)
		m.K8sVolumeInodes = &Metric{Enabled: true}
	},
	metricK8sVolumeInodesFree: func(m *KubeletStatsMetrics) {
		initKubeletStatsVolumeMetrics(m)
		m.K8sVolumeInodesFree = &Metric{Enabled: true}
	},
	metricK8sVolumeInodesUsed: func(m *KubeletStatsMetrics) {
		initKubeletStatsVolumeMetrics(m)
		m.K8sVolumeInodesUsed = &Metric{Enabled: true}
	},

	// KubeletStatsOptionalMetrics
	metricContainerUptime: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.ContainerUptime = &Metric{Enabled: true}
	},
	metricK8sContainerCPUNodeUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sContainerCPUNodeUtilization = &Metric{Enabled: true}
	},
	metricK8sContainerCPULimitUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sContainerCPULimitUtilization = &Metric{Enabled: true}
	},
	metricK8sContainerCPURequestUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sContainerCPURequestUtilization = &Metric{Enabled: true}
	},
	metricK8sContainerMemNodeUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sContainerMemNodeUtilization = &Metric{Enabled: true}
	},
	metricK8sContainerMemLimitUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sContainerMemLimitUtilization = &Metric{Enabled: true}
	},
	metricK8sContainerMemRequestUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sContainerMemRequestUtilization = &Metric{Enabled: true}
	},
	metricK8sNodeUptime: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sNodeUptime = &Metric{Enabled: true}
	},
	metricK8sPodCPUNodeUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodCPUNodeUtilization = &Metric{Enabled: true}
	},
	metricK8sPodCPULimitUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodCPULimitUtilization = &Metric{Enabled: true}
	},
	metricK8sPodCPURequestUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodCPURequestUtilization = &Metric{Enabled: true}
	},
	metricK8sPodMemNodeUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodMemNodeUtilization = &Metric{Enabled: true}
	},
	metricK8sPodMemLimitUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodMemLimitUtilization = &Metric{Enabled: true}
	},
	metricK8sPodMemRequestUtilization: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodMemRequestUtilization = &Metric{Enabled: true}
	},
	metricK8sPodUptime: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodUptime = &Metric{Enabled: true}
	},
	metricK8sPodVolumeUsage: func(m *KubeletStatsMetrics) {
		initKubeletStatsOptionalMetrics(m)
		m.K8sPodVolumeUsage = &Metric{Enabled: true}
	},
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
