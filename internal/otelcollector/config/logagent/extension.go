package logagent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

func extensionsConfig() Extensions {
	return Extensions{
		Extensions: common.ExtensionsConfig(),
		FileStorage: &FileStorage{
			Directory: otelcollector.CheckpointVolumePath,
		},
	}
}
