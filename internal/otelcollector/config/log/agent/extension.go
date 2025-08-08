package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

func extensionsConfig() Extensions {
	return Extensions{
		Extensions: config.DefaultExtensions(),
		FileStorage: &FileStorage{
			Directory: otelcollector.CheckpointVolumePath,
		},
	}
}
