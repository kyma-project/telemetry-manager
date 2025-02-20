package builder

import telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"

func BuildFluentBitFilesConfig(pipeline *telemetryv1alpha1.LogPipeline) map[string]string {
	var filesConfig map[string]string

	for _, file := range pipeline.Spec.Files {
		filesConfig[file.Name] = file.Content
	}

	return filesConfig
}
