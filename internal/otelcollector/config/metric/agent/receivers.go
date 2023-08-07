package agent

import (
	"fmt"
	"time"

	promcommonconfig "github.com/prometheus/common/config"
	prommodel "github.com/prometheus/common/model"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	promk8sdiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	promtargetgroup "github.com/prometheus/prometheus/discovery/targetgroup"
	promlabel "github.com/prometheus/prometheus/model/relabel"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func makeReceiversConfig(inputs inputSources) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusSelf = makePrometheusSelfConfig()
		receiversConfig.PrometheusAppPods = makePrometheusAppPodsConfig()
	}

	if inputs.runtime {
		receiversConfig.KubeletStats = makeKubeletStatsConfig()
	}

	if inputs.istio {
		receiversConfig.PrometheusIstio = makePrometheusIstioConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig() *KubeletStatsReceiver {
	const collectionInterval = "30s"
	const portKubelet = 10250
	return &KubeletStatsReceiver{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		Endpoint:           fmt.Sprintf("https://${env:%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod},
	}
}

func makePrometheusSelfConfig() *PrometheusReceiver {
	targets := []*promtargetgroup.Group{
		{
			Targets: []prommodel.LabelSet{
				{
					prommodel.AddressLabel: prommodel.LabelValue(fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.Metrics)),
				},
			},
		},
	}

	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
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

func makePrometheusAppPodsConfig() *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:        "app-pods",
					ScrapeInterval: prommodel.Duration(10 * time.Second),
					ServiceDiscoveryConfigs: []promdiscovery.Config{
						&promk8sdiscovery.SDConfig{
							Role:             promk8sdiscovery.RolePod,
							HTTPClientConfig: promcommonconfig.DefaultHTTPClientConfig,
						},
					},
					RelabelConfigs: []*promlabel.Config{
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_node_name"},
							Regex:        promlabel.MustNewRegexp(fmt.Sprintf("$%s", config.EnvVarCurrentNodeName)),
							Action:       promlabel.Keep,
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_scrape"},
							Regex:        promlabel.MustNewRegexp("true"),
							Action:       promlabel.Keep,
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_scheme"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("(https?)"),
							TargetLabel:  "__scheme__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_path"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("(.+)"),
							TargetLabel:  "__metrics_path__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("([^:]+)(?::\\d+)?;(\\d+)"),
							Replacement:  "$$1:$$2",
							TargetLabel:  "__address__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_phase"},
							Action:       promlabel.Drop,
							Regex:        promlabel.MustNewRegexp("Pending|Succeeded|Failed"),
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_container_init"},
							Action:       promlabel.Drop,
							Regex:        promlabel.MustNewRegexp("(true)"),
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_container_name"},
							Action:       promlabel.Drop,
							Regex:        promlabel.MustNewRegexp("(istio-proxy)"),
						},
					},
				},
			},
		},
	}
}

func makePrometheusIstioConfig() *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:        "istio-proxy",
					MetricsPath:    "/stats/prometheus",
					ScrapeInterval: prommodel.Duration(10 * time.Second),
					ServiceDiscoveryConfigs: []promdiscovery.Config{
						&promk8sdiscovery.SDConfig{
							Role:             promk8sdiscovery.RolePod,
							HTTPClientConfig: promcommonconfig.DefaultHTTPClientConfig,
						},
					},
					RelabelConfigs: []*promlabel.Config{
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_node_name"},
							Regex:        promlabel.MustNewRegexp(fmt.Sprintf("$%s", config.EnvVarCurrentNodeName)),
							Action:       promlabel.Keep,
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_container_name"},
							Action:       promlabel.Keep,
							Regex:        promlabel.MustNewRegexp("istio-proxy"),
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_container_port_name"},
							Action:       promlabel.Keep,
							Regex:        promlabel.MustNewRegexp("http-envoy-prom"),
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_phase"},
							Action:       promlabel.Drop,
							Regex:        promlabel.MustNewRegexp("Pending|Succeeded|Failed"),
						},
					},
					MetricRelabelConfigs: []*promlabel.Config{
						{
							SourceLabels: []prommodel.LabelName{"__name__"},
							Regex:        promlabel.MustNewRegexp("istio_.*"),
							Action:       promlabel.Keep,
						},
					},
				},
			},
		},
	}
}
