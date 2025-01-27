package agent

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

func makeExtensionsConfig() Extensions {
	return Extensions{
		BaseExtensions: config.DefaultBaseExtensions(),
		FileStorage: &FileStorage{
			//CreateDirectory: true,
			Directory: "/var/lib/otelcol",
		},
	}
}
