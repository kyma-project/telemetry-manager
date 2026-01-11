package builder

import telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"

func buildFluentBitFilesConfig(pipeline *telemetryv1beta1.LogPipeline) map[string]string {
	filesConfig := make(map[string]string)

	for _, file := range pipeline.Spec.FluentBitFiles {
		filesConfig[file.Name] = file.Content
	}

	return filesConfig
}
