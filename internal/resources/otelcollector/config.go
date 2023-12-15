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

	Deployment           DeploymentConfig
	Scaling              GatewayScalingConfig
	Istio                IstioConfig
	OTLPServiceName      string
	allowedPorts         []int32
	CanReceiveOpenCensus bool
}

type IstioConfig struct {
	Enabled      bool
	ExcludePorts string
}

func (cfg *GatewayConfig) WithScaling(s GatewayScalingConfig) *GatewayConfig {
	cfgCopy := *cfg
	cfgCopy.Scaling = s
	return &cfgCopy
}

func (cfg *GatewayConfig) WithCollectorConfig(collectorCfgYAML string, collectorEnvVars map[string][]byte) *GatewayConfig {
	cfgCopy := *cfg
	cfgCopy.CollectorConfig = collectorCfgYAML
	cfgCopy.CollectorEnvVars = collectorEnvVars
	return &cfgCopy
}

func (cfg *GatewayConfig) WithIstioConfig(excludePorts string, istioEnabled bool) *GatewayConfig {
	cfgCopy := *cfg
	istioConfg := IstioConfig{
		Enabled:      istioEnabled,
		ExcludePorts: excludePorts,
	}
	cfgCopy.Istio = istioConfg
	return &cfgCopy
}

func (cfg *GatewayConfig) WithAllowedPorts(ports []int32) *GatewayConfig {
	cfgCopy := *cfg

	cfgCopy.allowedPorts = ports
	return &cfgCopy

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

type GatewayScalingConfig struct {
	// Replicas specifies the number of gateway replicas.
	Replicas int32

	// ResourceRequirementsMultiplier is a coefficient affecting the CPU and memory resource limits for each replica.
	// This value is multiplied with a base resource requirement to calculate the actual CPU and memory limits.
	// A value of 1 applies the base limits; values greater than 1 increase those limits proportionally.
	ResourceRequirementsMultiplier int
}

type AgentConfig struct {
	Config
	allowedPorts []int32

	DaemonSet DaemonSetConfig
}

func (cfg *AgentConfig) WithCollectorConfig(collectorCfgYAML string) *AgentConfig {
	copy := *cfg
	copy.CollectorConfig = collectorCfgYAML
	return &copy
}

type DaemonSetConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	CPURequest        resource.Quantity
	MemoryLimit       resource.Quantity
	MemoryRequest     resource.Quantity
}

func (cfg *AgentConfig) WithAllowedPorts(ports []int32) *AgentConfig {
	cfgCopy := *cfg

	cfgCopy.allowedPorts = ports
	return &cfgCopy

}
