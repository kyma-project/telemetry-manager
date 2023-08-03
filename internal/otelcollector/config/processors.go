package config

type BaseProcessors struct {
	Batch         *BatchProcessor         `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiter          `yaml:"memory_limiter,omitempty"`
	K8sAttributes *K8sAttributesProcessor `yaml:"k8sattributes,omitempty"`
	Resource      *ResourceProcessor      `yaml:"resource,omitempty"`
}

type BatchProcessor struct {
	SendBatchSize    int    `yaml:"send_batch_size"`
	Timeout          string `yaml:"timeout"`
	SendBatchMaxSize int    `yaml:"send_batch_max_size"`
}

type MemoryLimiter struct {
	CheckInterval        string `yaml:"check_interval"`
	LimitPercentage      int    `yaml:"limit_percentage"`
	SpikeLimitPercentage int    `yaml:"spike_limit_percentage"`
}

type ExtractK8sMetadata struct {
	Metadata []string `yaml:"metadata"`
}

type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

type K8sAttributesProcessor struct {
	AuthType       string             `yaml:"auth_type"`
	Passthrough    bool               `yaml:"passthrough"`
	Extract        ExtractK8sMetadata `yaml:"extract"`
	PodAssociation []PodAssociations  `yaml:"pod_association"`
}

type AttributeAction struct {
	Action string `yaml:"action,omitempty"`
	Key    string `yaml:"key,omitempty"`
	Value  string `yaml:"value,omitempty"`
}

type ResourceProcessor struct {
	Attributes []AttributeAction `yaml:"attributes"`
}
