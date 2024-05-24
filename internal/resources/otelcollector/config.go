package otelcollector

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	BaseName                string
	Namespace               string
	ObserveBySelfMonitoring bool
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
