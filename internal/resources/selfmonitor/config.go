package selfmonitor

type Config struct {
	BaseName      string
	Namespace     string
	ComponentType string

	Deployment DeploymentConfig
}

type DeploymentConfig struct {
	Image             string
	PriorityClassName string
}
