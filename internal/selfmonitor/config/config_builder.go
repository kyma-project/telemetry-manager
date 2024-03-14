package config

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

func MakeConfig(scrapeNamespace string) Config {
	promConfig := Config{}
	promConfig.GlobalConfig = makeGlobalConfig()
	promConfig.AlertingConfig = makeAlertConfig()
	promConfig.RuleFiles = []string{"/etc/prometheus/alerting_rules.yml"}
	promConfig.ScrapeConfigs = makeScrapeConfig(scrapeNamespace)
	return promConfig
}

func makeGlobalConfig() GlobalConfig {
	return GlobalConfig{
		ScraperInterval:    10 * time.Second,
		EvaluationInterval: 10 * time.Second,
	}
}

func makeAlertConfig() AlertingConfig {
	return AlertingConfig{
		AlertManagers: []AlertManagerConfig{{
			StaticConfigs: []AlertManagerStaticConfig{{
				Targets: []string{fmt.Sprintf("localhost:%d", ports.AlertingPort)},
			}},
		}},
	}
}

func makeScrapeConfig(scrapeNamespace string) []ScrapeConfig {
	return []ScrapeConfig{
		{
			JobName: "kubernetes-service-endpoints",
			RelabelConfigs: []RelabelConfig{
				{
					SourceLabels: []string{"__meta_kubernetes_namespace"},
					Action:       Keep,
					Regex:        scrapeNamespace,
				},
				{
					SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
					Action:       Keep,
					Regex:        "true",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_endpoints_label_telemetry_kyma_project_io_self_monitor"},
					Action:       Keep,
					Regex:        "enabled",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_path"},
					Action:       Replace,
					TargetLabel:  "__metrics_path__",
					Regex:        "true",
				},
				{
					SourceLabels: []string{"__address__", "__meta_kubernetes_service_annotation_prometheus_io_port"},
					Action:       Replace,
					TargetLabel:  "__address__",
					Regex:        "(.+?)(?::\\d+)?;(\\d+)",
					Replacement:  "$1:$2",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_namespace"},
					Action:       Replace,
					TargetLabel:  "namespace",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_service_name"},
					Action:       Replace,
					TargetLabel:  "service",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_pod_node_name"},
					Action:       Replace,
					TargetLabel:  "node",
				},
			},
			MetricRelabelConfigs: []RelabelConfig{
				{
					SourceLabels: []string{"__name__"},
					Action:       Keep,
					Regex:        "(otelcol_.*)",
				},
			},
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{
				Role:       RoleEndpoints,
				Namespaces: Names{Name: []string{scrapeNamespace}},
			}},
		},
	}
}
