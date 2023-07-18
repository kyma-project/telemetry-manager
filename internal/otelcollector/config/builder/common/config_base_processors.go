package common

type BaseProcessorsConfig struct {
	Batch         *BatchProcessorConfig         `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiterConfig          `yaml:"memory_limiter,omitempty"`
	K8sAttributes *K8sAttributesProcessorConfig `yaml:"k8sattributes,omitempty"`
	Resource      *ResourceProcessorConfig      `yaml:"resource,omitempty"`
}

type BatchProcessorConfig struct {
	SendBatchSize    int    `yaml:"send_batch_size"`
	Timeout          string `yaml:"timeout"`
	SendBatchMaxSize int    `yaml:"send_batch_max_size"`
}

type MemoryLimiterConfig struct {
	CheckInterval        string `yaml:"check_interval"`
	LimitPercentage      int    `yaml:"limit_percentage"`
	SpikeLimitPercentage int    `yaml:"spike_limit_percentage"`
}

type ExtractK8sMetadataConfig struct {
	Metadata []string `yaml:"metadata"`
}

type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

type K8sAttributesProcessorConfig struct {
	AuthType       string                   `yaml:"auth_type"`
	Passthrough    bool                     `yaml:"passthrough"`
	Extract        ExtractK8sMetadataConfig `yaml:"extract"`
	PodAssociation []PodAssociations        `yaml:"pod_association"`
}

type AttributeAction struct {
	Action string `yaml:"action,omitempty"`
	Key    string `yaml:"key,omitempty"`
	Value  string `yaml:"value,omitempty"`
}

type ResourceProcessorConfig struct {
	Attributes []AttributeAction `yaml:"attributes"`
}
