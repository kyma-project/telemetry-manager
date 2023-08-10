package agent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

func makeReceiversConfig(inputs inputSources) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusSelf = makePrometheusSelfConfig()
		receiversConfig.PrometheusAppPods = makePrometheusAppPodsConfig()
		receiversConfig.PrometheusAppServices = makePrometheusAppServicesConfig()
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
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:        "opentelemetry-collector",
					ScrapeInterval: 10 * time.Second,
					StaticDiscoveryConfigs: []StaticDiscoveryConfig{
						{
							Targets: []string{fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.Metrics)},
						},
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
					ScrapeInterval: 10 * time.Second,
					KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{
						{
							Role: RolePod,
						},
					},
					RelabelConfigs: []RelabelConfig{
						{
							SourceLabels: []string{"__meta_kubernetes_pod_node_name"},
							Regex:        fmt.Sprintf("$%s", config.EnvVarCurrentNodeName),
							Action:       Keep,
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_annotation_prometheus_io_scrape"},
							Regex:        "true",
							Action:       Keep,
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_annotation_prometheus_io_scheme"},
							Action:       Replace,
							Regex:        "(https?)",
							TargetLabel:  "__scheme__",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_annotation_prometheus_io_path"},
							Action:       Replace,
							Regex:        "(.+)",
							TargetLabel:  "__metrics_path__",
						},
						{
							SourceLabels: []string{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
							Action:       Replace,
							Regex:        "([^:]+)(?::\\d+)?;(\\d+)",
							Replacement:  "$$1:$$2",
							TargetLabel:  "__address__",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_phase"},
							Action:       Drop,
							Regex:        "Pending|Succeeded|Failed",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_container_init"},
							Action:       Drop,
							Regex:        "(true)",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
							Action:       Drop,
							Regex:        "(istio-proxy)",
						},
					},
				},
			},
		},
	}
}

func makePrometheusAppServicesConfig() *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:        "app-services",
					ScrapeInterval: 10 * time.Second,
					KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{
						{
							Role: RoleEndpoints,
						},
					},
					RelabelConfigs: []RelabelConfig{
						{
							SourceLabels: []string{"__meta_kubernetes_endpoint_node_name"},
							Regex:        fmt.Sprintf("$%s", config.EnvVarCurrentNodeName),
							Action:       Keep,
						},
						{
							SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
							Regex:        "true",
							Action:       Keep,
						},
						{
							SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scheme"},
							Action:       Replace,
							Regex:        "(https?)",
							TargetLabel:  "__scheme__",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_path"},
							Action:       Replace,
							Regex:        "(.+)",
							TargetLabel:  "__metrics_path__",
						},
						{
							SourceLabels: []string{"__address__", "__meta_kubernetes_service_annotation_prometheus_io_port"},
							Action:       Replace,
							Regex:        "([^:]+)(?::\\d+)?;(\\d+)",
							Replacement:  "$$1:$$2",
							TargetLabel:  "__address__",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_phase"},
							Action:       Drop,
							Regex:        "Pending|Succeeded|Failed",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_container_init"},
							Action:       Drop,
							Regex:        "(true)",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
							Action:       Drop,
							Regex:        "(istio-proxy)",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_service_name"},
							Action:       Replace,
							TargetLabel:  "service",
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
					ScrapeInterval: 10 * time.Second,
					KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{
						{
							Role: RolePod,
						},
					},
					RelabelConfigs: []RelabelConfig{
						{
							SourceLabels: []string{"__meta_kubernetes_pod_node_name"},
							Regex:        fmt.Sprintf("$%s", config.EnvVarCurrentNodeName),
							Action:       Keep,
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
							Action:       Keep,
							Regex:        "istio-proxy",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_container_port_name"},
							Action:       Keep,
							Regex:        "http-envoy-prom",
						},
						{
							SourceLabels: []string{"__meta_kubernetes_pod_phase"},
							Action:       Drop,
							Regex:        "Pending|Succeeded|Failed",
						},
					},
					MetricRelabelConfigs: []RelabelConfig{
						{
							SourceLabels: []string{"__name__"},
							Regex:        "istio_.*",
							Action:       Keep,
						},
					},
				},
			},
		},
	}
}
