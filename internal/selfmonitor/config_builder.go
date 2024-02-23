package selfmonitor

import (
	"time"

	"github.com/kyma-project/telemetry-manager/internal/prometheus"
)

func MakeConfig() monitoringConfig {
	promConfig := monitoringConfig{}
	promConfig.GlobalConfig = globalConfigBuilder()
	promConfig.AlertingConfig = alertConfigBuilder()
	promConfig.RuleFiles = []string{"/etc/prometheus/prometheus.rules"}
	promConfig.ScrapeConfigs = scrapeConfigBuilder()
	return promConfig
}

func globalConfigBuilder() prometheus.GlobalConfig {
	return prometheus.GlobalConfig{
		ScraperInterval:    10 * time.Second,
		EvaluationInterval: 10 * time.Second,
	}
}

func alertConfigBuilder() prometheus.AlertingConfig {
	return prometheus.AlertingConfig{
		AlertManagers: []prometheus.AlertManagerConfig{{
			StaticConfigs: []prometheus.StaticConfig{{
				Targets: []string{"localhost:9093"},
			}},
		}},
	}
}

func scrapeConfigBuilder() []prometheus.ScrapeConfig {
	return []prometheus.ScrapeConfig{
		{
			JobName:                    "kubernetes-service-endpoints",
			RelabelConfigs:             []prometheus.RelabelConfig{prometheus.KeepServiceAnnotations()},
			KubernetesDiscoveryConfigs: []prometheus.KubernetesDiscoveryConfig{{Role: prometheus.RoleEndpoints}},
		},
	}
}
