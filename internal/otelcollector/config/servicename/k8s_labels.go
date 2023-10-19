package servicename

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func ExtractLabels() []config.ExtractLabel {
	return []config.ExtractLabel{
		{
			From:    "pod",
			Key:     "app.kubernetes.io/name",
			TagName: "kyma.kubernetes_io_app_name",
		},
		{
			From:    "pod",
			Key:     "app",
			TagName: "kyma.app_name",
		},
	}
}
