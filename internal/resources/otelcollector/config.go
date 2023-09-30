package otelcollector

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	BaseName         string
	Namespace        string
	CollectorConfig  string
	CollectorEnvVars map[string][]byte
}

type GatewayConfig struct {
	Config

	Deployment       DeploymentConfig
	OTLPServiceName  string
	CreateOpenCensus bool
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
