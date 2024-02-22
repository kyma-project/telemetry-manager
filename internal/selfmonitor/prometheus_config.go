package selfmonitor

import "time"

type prometheusConfig struct {
	GlobalConfig   globalConfig   `yaml:"global"`
	AlertingConfig alertingConfig `yaml:"alerting,omitempty"`
	RuleFiles      []string       `yaml:"rule_files,omitempty"`
	ScrapeConfigs  []scrapeConfig `yaml:"scrape_configs,omitempty"`
}

type globalConfig struct {
	ScraperInterval    time.Duration `yaml:"scrape_interval"`
	EvaluationInterval time.Duration `yaml:"evaluation_interval"`
}

type alertingConfig struct {
	AlertManagers []alertManagerConfig `yaml:"alertmanagers"`
}

type alertManagerConfig struct {
	StaticConfigs []staticConfig `yaml:"static_configs"`
}
type staticConfig struct {
	Targets []string `yaml:"targets"`
}

type scrapeConfig struct {
	JobName              string          `yaml:"job_name"`
	RelabelConfigs       []relabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []relabelConfig `yaml:"metric_relabel_configs,omitempty"`

	StaticDiscoveryConfigs     []StaticDiscoveryConfig     `yaml:"static_configs,omitempty"`
	KubernetesDiscoveryConfigs []KubernetesDiscoveryConfig `yaml:"kubernetes_sd_configs,omitempty"`
}
type relabelConfig struct {
	SourceLabels []string      `yaml:"source_labels,flow,omitempty"`
	Separator    string        `yaml:"separator,omitempty"`
	Regex        string        `yaml:"regex,omitempty"`
	Modulus      uint64        `yaml:"modulus,omitempty"`
	TargetLabel  string        `yaml:"target_label,omitempty"`
	Replacement  string        `yaml:"replacement,omitempty"`
	Action       RelabelAction `yaml:"action,omitempty"`
}

type RelabelAction string

type StaticDiscoveryConfig struct {
	Targets []string `yaml:"targets"`
}

type KubernetesDiscoveryConfig struct {
	Role Role `yaml:"role"`
}
type Role string

const (
	RoleEndpoints Role = "endpoints"
	RolePod       Role = "pod"
)
