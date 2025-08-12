package agent

import (
	"path/filepath"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const checkpointVolumePathSubdirectory = "telemetry-log-agent/file-log-receiver"

func extensionsConfig() Extensions {
	return Extensions{
		Extensions: config.DefaultExtensions(),
		FileStorage: &FileStorage{
			CreateDirectory: true,
			Directory:       filepath.Join(otelcollector.CheckpointVolumePath, checkpointVolumePathSubdirectory),
		},
	}
}
