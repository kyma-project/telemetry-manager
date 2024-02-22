package selfmonitor

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	BaseName         string
	Namespace        string
	prometheusConfig string
}

type DeploymentConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	CPURequest        resource.Quantity
	MemoryLimit       resource.Quantity
	MemoryRequest     resource.Quantity
}

type PrometheusDeploymentConfig struct {
	Config

	Deployment   DeploymentConfig
	allowedPorts []int32
	Replicas     int32
}

func (promCfg *PrometheusDeploymentConfig) WithPrometheusConfig(prometheusCfgYAML string) *PrometheusDeploymentConfig {
	cfgCopy := *promCfg
	cfgCopy.prometheusConfig = prometheusCfgYAML
	return &cfgCopy
}

func (promCfg *PrometheusDeploymentConfig) WithAllowedPorts() *PrometheusDeploymentConfig {
	cfgCopy := *promCfg
	cfgCopy.allowedPorts = []int32{int32(9090)}
	return &cfgCopy
}
