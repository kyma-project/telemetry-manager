package agent

import (
	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	prommodel "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	promtargetgroup "github.com/prometheus/prometheus/discovery/targetgroup"
	"time"
)

func makeReceiversConfig(pipelines []v1alpha1.MetricPipeline) config.ReceiversConfig {
	enableRuntimeMetrics := false
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Runtime.Enabled {
			enableRuntimeMetrics = true
		}
	}

	var receiversConfig config.ReceiversConfig

	receiversConfig.PrometheusSelf = makePrometheusSelfConfig()

	if enableRuntimeMetrics {
		receiversConfig.KubeletStats = makeKubeletStatsConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig() *config.KubeletStatsReceiverConfig {
	const collectionInterval = "30s"
	return &config.KubeletStatsReceiverConfig{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		Endpoint:           "https://${env:MY_NODE_NAME}:10250",
		InsecureSkipVerify: true,
		MetricGroups:       []config.MetricGroupType{config.MetricGroupTypeContainer, config.MetricGroupTypePod},
	}
}

func makePrometheusSelfConfig() *config.PrometheusReceiverConfig {
	targets := []*promtargetgroup.Group{
		{
			Targets: []prommodel.LabelSet{
				{
					prommodel.AddressLabel: "${MY_POD_IP}:8888",
				},
			},
		},
	}

	return &config.PrometheusReceiverConfig{
		Config: promconfig.Config{
			ScrapeConfigs: []*promconfig.ScrapeConfig{
				{
					JobName:        "opentelemetry-collector",
					ScrapeInterval: prommodel.Duration(10 * time.Second),
					ServiceDiscoveryConfigs: []promdiscovery.Config{
						promdiscovery.StaticConfig(targets),
					},
				},
			},
		},
	}
}
