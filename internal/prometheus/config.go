package prometheus

import "time"

type ScrapeConfig struct {
	JobName              string          `yaml:"job_name"`
	SampleLimit          int             `yaml:"sample_limit,omitempty"`
	ScrapeInterval       time.Duration   `yaml:"scrape_interval,omitempty"`
	MetricsPath          string          `yaml:"metrics_path,omitempty"`
	RelabelConfigs       []RelabelConfig `yaml:"relabel_configs,omitempty"`
	MetricRelabelConfigs []RelabelConfig `yaml:"metric_relabel_configs,omitempty"`

	StaticDiscoveryConfigs     []StaticDiscoveryConfig     `yaml:"static_configs,omitempty"`
	KubernetesDiscoveryConfigs []KubernetesDiscoveryConfig `yaml:"kubernetes_sd_configs,omitempty"`

	TLSConfig *TLSConfig `yaml:"tls_config,omitempty"`
}

type TLSConfig struct {
	CAFile             string `yaml:"ca_file,omitempty"`
	CertFile           string `yaml:"cert_file,omitempty"`
	KeyFile            string `yaml:"key_file,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

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
	Replace RelabelAction = "replace"
	Keep    RelabelAction = "keep"
	Drop    RelabelAction = "drop"
)
