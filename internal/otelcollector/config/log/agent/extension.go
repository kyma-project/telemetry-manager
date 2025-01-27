package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

func makeExtensionsConfig() Extensions {
	return Extensions{
		BaseExtensions: config.DefaultBaseExtensions(),
		FileStorage: &FileStorage{
			Directory: otelcollector.CheckpointVolumePath,
		},
	}
}
