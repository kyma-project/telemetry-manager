package config

type BaseProcessors struct {
	Batch         *BatchProcessor `yaml:"batch,omitempty"`
	MemoryLimiter *MemoryLimiter  `yaml:"memory_limiter,omitempty"`
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

type K8sAttributesProcessor struct {
	AuthType       string             `yaml:"auth_type"`
	Passthrough    bool               `yaml:"passthrough"`
	Extract        ExtractK8sMetadata `yaml:"extract"`
	PodAssociation []PodAssociations  `yaml:"pod_association"`
}

type ExtractK8sMetadata struct {
	Metadata []string       `yaml:"metadata"`
	Labels   []ExtractLabel `yaml:"labels"`
}

type ExtractLabel struct {
	From    string `yaml:"from"`
	Key     string `yaml:"key"`
	TagName string `yaml:"tag_name"`
}

type PodAssociations struct {
	Sources []PodAssociation `yaml:"sources"`
}

type PodAssociation struct {
	From string `yaml:"from"`
	Name string `yaml:"name,omitempty"`
}

type ResourceProcessor struct {
	Attributes []AttributeAction `yaml:"attributes"`
}

type AttributeAction struct {
	Action       string `yaml:"action,omitempty"`
	Key          string `yaml:"key,omitempty"`
	Value        string `yaml:"value,omitempty"`
	RegexPattern string `yaml:"pattern,omitempty"`
}

type TransformProcessorStatements struct {
	Context    string   `yaml:"context"`
	Statements []string `yaml:"statements"`
}
