package config

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/ports"
)

func MakeConfig() Config {
	promConfig := Config{}
	promConfig.GlobalConfig = makeGlobalConfig()
	promConfig.AlertingConfig = makeAlertConfig()
	promConfig.RuleFiles = []string{"/etc/prometheus/prometheus.rules"}
	promConfig.ScrapeConfigs = makeScrapeConfig()
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

func makeScrapeConfig() []ScrapeConfig {
	return []ScrapeConfig{
		{
			JobName: "kubernetes-service-endpoints",
			RelabelConfigs: []RelabelConfig{{
				SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
				Regex:        "true",
				Action:       Keep,
			}},
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RoleEndpoints}},
		},
	}
}
