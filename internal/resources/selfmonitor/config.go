package selfmonitor

type Config struct {
	BaseName  string
	Namespace string

	Deployment DeploymentConfig
}

type DeploymentConfig struct {
	Image             string
	PriorityClassName string
}
