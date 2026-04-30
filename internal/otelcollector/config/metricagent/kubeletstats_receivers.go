package metricagent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func kubeletStatsReceiver(runtimeResources runtimeResourceSources, collectionInterval time.Duration) *KubeletStatsReceiverConfig {
	const portKubelet = 10250

	return &KubeletStatsReceiverConfig{
		CollectionInterval: collectionInterval.String(),
		AuthType:           "serviceAccount",
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${%s}:%d", common.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       kubeletStatsMetricGroups(runtimeResources),
		Metrics: KubeletStatsMetrics{
			ContainerCPUUsage:            Metric{Enabled: true},
			K8sPodCPUUsage:               Metric{Enabled: true},
			K8sNodeCPUUsage:              Metric{Enabled: true},
			K8sNodeCPUTime:               Metric{Enabled: false},
			K8sNodeMemoryMajorPageFaults: Metric{Enabled: false},
			K8sNodeMemoryPageFaults:      Metric{Enabled: false},
		},
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

func kubeletStatsMetricGroups(runtimeResources runtimeResourceSources) []MetricGroupType {
	var metricGroups []MetricGroupType

	if runtimeResources.container {
		metricGroups = append(metricGroups, MetricGroupTypeContainer)
	}

	if runtimeResources.pod {
		metricGroups = append(metricGroups, MetricGroupTypePod)
	}

	if runtimeResources.node {
		metricGroups = append(metricGroups, MetricGroupTypeNode)
	}

	if runtimeResources.volume {
		metricGroups = append(metricGroups, MetricGroupTypeVolume)
	}

	return metricGroups
}
