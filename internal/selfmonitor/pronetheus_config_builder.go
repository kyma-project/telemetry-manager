package selfmonitor

import "time"

func MakeConfig() prometheusConfig {
	promConfig := prometheusConfig{}
	promConfig.GlobalConfig = globalConfigBuilder()
	promConfig.AlertingConfig = alertConfigBuilder()
	promConfig.RuleFiles = []string{"/etc/prometheus/prometheus.rules"}
	promConfig.ScrapeConfigs = scrapeConfigBuilder()
	return promConfig
}

func globalConfigBuilder() globalConfig {
	return globalConfig{
		ScraperInterval:    10 * time.Second,
		EvaluationInterval: 10 * time.Second,
	}
}

func alertConfigBuilder() alertingConfig {
	return alertingConfig{
		AlertManagers: []alertManagerConfig{{
			StaticConfigs: []staticConfig{{
				Targets: []string{"localhost:9093"},
			}},
		}},
	}
}

func scrapeConfigBuilder() []scrapeConfig {
	return []scrapeConfig{
		{
			JobName:                    "kubernetes-service-endpoints",
			RelabelConfigs:             relabelConfigBuilder(),
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RoleEndpoints}}},
	}
}

func relabelConfigBuilder() []relabelConfig {
	return []relabelConfig{{SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scrape"}, Regex: "true", Action: "keep"}}
}
