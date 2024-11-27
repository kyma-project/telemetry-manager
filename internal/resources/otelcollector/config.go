package otelcollector

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	defaultBaseCPULimit         = resource.MustParse("700m")
	defaultDynamicCPULimit      = resource.MustParse("500m")
	defaultBaseMemoryLimit      = resource.MustParse("500Mi")
	defaultDynamicMemoryLimit   = resource.MustParse("1500Mi")
	defaultBaseCPURequest       = resource.MustParse("100m")
	defaultDynamicCPURequest    = resource.MustParse("100m")
	defaultBaseMemoryRequest    = resource.MustParse("32Mi")
	defaultDynamicMemoryRequest = resource.MustParse("0")
)

type Config struct {
	BaseName  string
	Namespace string
}

type GatewayConfig struct {
	Config

	Deployment      DeploymentConfig
	OTLPServiceName string
}

type DeploymentConfig struct {
	Image                string
	PriorityClassName    string
	BaseCPULimit         resource.Quantity
	DynamicCPULimit      resource.Quantity
	BaseMemoryLimit      resource.Quantity
	DynamicMemoryLimit   resource.Quantity
	BaseCPURequest       resource.Quantity
	DynamicCPURequest    resource.Quantity
	BaseMemoryRequest    resource.Quantity
	DynamicMemoryRequest resource.Quantity
}

type DeploymentConfigOption func(*DeploymentConfig)

func WithImage(image string) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.Image = image
	}
}

func WithPriorityClassName(name string) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.PriorityClassName = name
	}
}

func WithBaseCPULimit(limit resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.BaseCPULimit = limit
	}
}

func WithDynamicCPULimit(limit resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.DynamicCPULimit = limit
	}
}

func WithBaseMemoryLimit(limit resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.BaseMemoryLimit = limit
	}
}

func WithDynamicMemoryLimit(limit resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.DynamicMemoryLimit = limit
	}
}

func WithBaseCPURequest(request resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.BaseCPURequest = request
	}
}

func WithDynamicCPURequest(request resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.DynamicCPURequest = request
	}
}

func WithBaseMemoryRequest(request resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.BaseMemoryRequest = request
	}
}

func WithDynamicMemoryRequest(request resource.Quantity) DeploymentConfigOption {
	return func(c *DeploymentConfig) {
		c.DynamicMemoryRequest = request
	}
}

func NewDeploymentConfig(opts ...DeploymentConfigOption) DeploymentConfig {
	config := DeploymentConfig{
		BaseCPULimit:         defaultBaseCPULimit,
		DynamicCPULimit:      defaultDynamicCPULimit,
		BaseMemoryLimit:      defaultBaseMemoryLimit,
		DynamicMemoryLimit:   defaultDynamicMemoryLimit,
		BaseCPURequest:       defaultBaseCPURequest,
		DynamicCPURequest:    defaultDynamicCPURequest,
		BaseMemoryRequest:    defaultBaseMemoryRequest,
		DynamicMemoryRequest: defaultDynamicMemoryRequest,
	}

	for _, opt := range opts {
		opt(&config)
	}

	return config
}

type AgentConfig struct {
	Config

	DaemonSet DaemonSetConfig
}

type DaemonSetConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	CPURequest        resource.Quantity
	MemoryLimit       resource.Quantity
	MemoryRequest     resource.Quantity
}
