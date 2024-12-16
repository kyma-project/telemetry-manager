package v1alpha1

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

const (
	maxPipelines = 5
)

// TODO: Align max log pipeline enforcement with the method used in the TracePipeline/MetricPipeline controllers,
// replacing the current validating webhook approach.
func validatePipelineLimit(logPipeline *telemetryv1alpha1.LogPipeline, logPipelines *telemetryv1alpha1.LogPipelineList) error {
	if isNewPipeline(logPipeline, logPipelines) && len(logPipelines.Items) >= maxPipelines {
		return fmt.Errorf("the maximum number of log pipelines is %d", maxPipelines)
	}

	return nil
}

func isNewPipeline(logPipeline *telemetryv1alpha1.LogPipeline, logPipelines *telemetryv1alpha1.LogPipelineList) bool {
	for _, pipeline := range logPipelines.Items {
		if pipeline.Name == logPipeline.Name {
			return false
		}
	}

	return true
}
