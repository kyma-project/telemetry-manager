package selfmonitor

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

type Config struct {
	BaseName          string
	Namespace         string
	SelfMonitorConfig string

	Deployment DeploymentConfig
}

type DeploymentConfig struct {
	Image             string
	PriorityClassName string
	CPULimit          resource.Quantity
	CPURequest        resource.Quantity
	MemoryLimit       resource.Quantity
	MemoryRequest     resource.Quantity
}
