package config

import (
	"time"
)

type BuilderConfig struct {
	ScrapeNamespace string
	WebhookURL      string
	WebhookScheme   string
}

func MakeConfig(builderCfg BuilderConfig) Config {
	promConfig := Config{}
	promConfig.GlobalConfig = makeGlobalConfig()
	promConfig.AlertingConfig = makeAlertConfig(builderCfg.WebhookURL, builderCfg.WebhookScheme)
	promConfig.RuleFiles = []string{"/etc/prometheus/alerting_rules.yml"}
	promConfig.ScrapeConfigs = makeScrapeConfig(builderCfg.ScrapeNamespace)
	return promConfig
}

func makeGlobalConfig() GlobalConfig {
	return GlobalConfig{
		ScraperInterval:    10 * time.Second,
		EvaluationInterval: 10 * time.Second,
	}
}

func makeAlertConfig(webhookURL, webhookScheme string) AlertingConfig {
	return AlertingConfig{
		AlertManagers: []AlertManagerConfig{{
			Scheme: webhookScheme,
			StaticConfigs: []AlertManagerStaticConfig{{
				Targets: []string{webhookURL},
			}},
			TLSConfig: TLSConfig{
				InsecureSkipVerify: true,
			},
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
					Regex:        "(.+)",
				},
				{
					SourceLabels: []string{"__address__", "__meta_kubernetes_service_annotation_prometheus_io_port"},
					Action:       Replace,
					TargetLabel:  "__address__",
					Regex:        "(.+?)(?::\\d+)?;(\\d+)",
					Replacement:  "$1:$2",
				},
				//{
				//	SourceLabels: []string{"__name__", "exporter"},
				//	Action:       Replace,
				//	Regex:        "otelcol_\\d+;\\d+/(.+)",
				//	TargetLabel:  "pipeline",
				//},
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
					Regex:        "(otelcol_.*|fluentbit_.*|telemetry_.*)",
				},
				{
					SourceLabels: []string{"__name__", "name"},
					Action:       Replace,
					Regex:        "fluentbit_.+;(.+)",
					TargetLabel:  "pipeline_name",
				},
				{
					SourceLabels: []string{"__name__", "exporter"},
					Action:       Replace,
					Regex:        "otelcol_.+;(.+)/(.+)",
					TargetLabel:  "pipeline_name",
					Replacement:  "$2",
				},
			},
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{
				Role:       RoleEndpoints,
				Namespaces: Names{Name: []string{scrapeNamespace}},
			}},
		},
	}
}
