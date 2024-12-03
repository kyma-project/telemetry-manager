package otelcollector

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	BaseName  string
	Namespace string
}

type GatewayConfig struct {
	Config

	Image             string
	PriorityClassName string
	OTLPServiceName   string
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
