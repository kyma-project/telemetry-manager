package config

import (
	"time"
)

type Config struct {
	GlobalConfig   GlobalConfig   `yaml:"global"`
	AlertingConfig AlertingConfig `yaml:"alerting,omitempty"`
	RuleFiles      []string       `yaml:"rule_files,omitempty"`
	ScrapeConfigs  []ScrapeConfig `yaml:"scrape_configs,omitempty"`
}

type GlobalConfig struct {
	ScraperInterval    time.Duration `yaml:"scrape_interval"`
	EvaluationInterval time.Duration `yaml:"evaluation_interval"`
}

type AlertingConfig struct {
	AlertManagers []AlertManagerConfig `yaml:"alertmanagers"`
}

type AlertManagerConfig struct {
	StaticConfigs []AlertManagerStaticConfig `yaml:"static_configs"`
}

type AlertManagerStaticConfig struct {
	Targets []string `yaml:"targets"`
}

type ScrapeConfig struct {
	JobName              string          `yaml:"job_name"`
	SampleLimit          int             `yaml:"sample_limit,omitempty"`
	ScrapeInterval       time.Duration   `yaml:"scrape_interval,omitempty"`
	MetricsPath          string          `yaml:"metrics_path,omitempty"`
	RelabelConfigs       []RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []RelabelConfig `yaml:"metric_relabel_configs,omitempty"`

	KubernetesDiscoveryConfigs []KubernetesDiscoveryConfig `yaml:"kubernetes_sd_configs,omitempty"`
}

type KubernetesDiscoveryConfig struct {
	Role Role `yaml:"role"`
}

type Role string

const (
	RoleEndpoints Role = "endpoints"
)

type RelabelConfig struct {
	SourceLabels []string      `yaml:"source_labels,flow,omitempty"`
	Separator    string        `yaml:"separator,omitempty"`
	Regex        string        `yaml:"regex,omitempty"`
	Modulus      uint64        `yaml:"modulus,omitempty"`
	TargetLabel  string        `yaml:"target_label,omitempty"`
	Replacement  string        `yaml:"replacement,omitempty"`
	Action       RelabelAction `yaml:"action,omitempty"`
}

type RelabelAction string

const (
	Keep RelabelAction = "keep"
)
