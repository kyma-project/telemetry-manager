package selfmonitor

import "github.com/kyma-project/telemetry-manager/internal/config"

type Config struct {
	config.Global

	Image             string
	PriorityClassName string
}
