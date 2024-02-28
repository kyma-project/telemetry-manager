package selfmonitor

import (
	"time"

	"github.com/kyma-project/telemetry-manager/internal/prometheus"
)

func MakeConfig() monitoringConfig {
	promConfig := monitoringConfig{}
	promConfig.GlobalConfig = makeGlobalConfig()
	promConfig.AlertingConfig = makeAlertConfig()
	promConfig.RuleFiles = []string{"/etc/prometheus/prometheus.rules"}
	promConfig.ScrapeConfigs = makeScrapeConfig()
	return promConfig
}

func makeGlobalConfig() prometheus.GlobalConfig {
	return prometheus.GlobalConfig{
		ScraperInterval:    10 * time.Second,
		EvaluationInterval: 10 * time.Second,
	}
}

func makeAlertConfig() prometheus.AlertingConfig {
	return prometheus.AlertingConfig{
		AlertManagers: []prometheus.AlertManagerConfig{{
			StaticConfigs: []prometheus.StaticConfig{{
				Targets: []string{"localhost:9093"},
			}},
		}},
	}
}

func makeScrapeConfig() []prometheus.ScrapeConfig {
	return []prometheus.ScrapeConfig{
		{
			JobName: "kubernetes-service-endpoints",
			RelabelConfigs: []prometheus.RelabelConfig{{
				SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
				Regex:        "true",
				Action:       prometheus.Keep,
			}},
			KubernetesDiscoveryConfigs: []prometheus.KubernetesDiscoveryConfig{{Role: prometheus.RoleEndpoints}},
		},
	}
}
