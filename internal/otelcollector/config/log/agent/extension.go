package agent

import "github.com/kyma-project/telemetry-manager/internal/otelcollector/config"

func makeExtensionsConfig() Extensions {
	return Extensions{
		Extensions: config.DefaultExtensions(),
		FileStorage: &FileStorage{
			Directory: "/var/log/otel",
		},
	}
}
