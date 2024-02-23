package selfmonitor

import (
	"github.com/kyma-project/telemetry-manager/internal/prometheus"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	BaseName         string
	Namespace        string
	monitoringConfig string

	Deployment   DeploymentConfig
	allowedPorts []int32
}

type DeploymentConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	CPURequest        resource.Quantity
	MemoryLimit       resource.Quantity
	MemoryRequest     resource.Quantity
}

type monitoringConfig struct {
	GlobalConfig   prometheus.GlobalConfig   `yaml:"global"`
	AlertingConfig prometheus.AlertingConfig `yaml:"alerting,omitempty"`
	RuleFiles      []string                  `yaml:"rule_files,omitempty"`
	ScrapeConfigs  []prometheus.ScrapeConfig `yaml:"scrape_configs,omitempty"`
}

func (promCfg *Config) WithMonitoringConfig(monitoringCfgYAML string) *Config {
	cfgCopy := *promCfg
	cfgCopy.monitoringConfig = monitoringCfgYAML
	return &cfgCopy
}
