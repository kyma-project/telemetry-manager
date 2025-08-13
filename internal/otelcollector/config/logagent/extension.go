package logagent

import (
	"path/filepath"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
)

const checkpointVolumePathSubdirectory = "telemetry-log-agent/file-log-receiver"

func extensionsConfig() Extensions {
	return Extensions{
		Extensions: common.ExtensionsConfig(),
		FileStorage: &FileStorage{
			CreateDirectory: true,
			Directory:       filepath.Join(otelcollector.CheckpointVolumePath, checkpointVolumePathSubdirectory),
		},
	}
}
