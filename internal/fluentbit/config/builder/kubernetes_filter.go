package builder

import (
	"fmt"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func createKubernetesFilter(pipeline *telemetryv1beta1.LogPipeline) string {
	runtimeInput := &telemetryv1beta1.LogPipelineRuntimeInput{}
	if pipeline.Spec.Input.Runtime != nil {
		runtimeInput = pipeline.Spec.Input.Runtime
	}

	keepAnnotations := false
	if runtimeInput.FluentBitKeepAnnotations != nil {
		keepAnnotations = *runtimeInput.FluentBitKeepAnnotations
	}

	keepLabels := true
	if runtimeInput.FluentBitDropLabels != nil {
		keepLabels = !*runtimeInput.FluentBitDropLabels
	}

	keepOriginalBody := true
	if runtimeInput.KeepOriginalBody != nil {
		keepOriginalBody = *runtimeInput.KeepOriginalBody
	}

	return NewFilterSectionBuilder().
		AddConfigParam("name", "kubernetes").
		AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name)).
		AddConfigParam("merge_log", "on").
		AddConfigParam("k8s-logging.parser", "on").
		AddConfigParam("k8s-logging.exclude", "off").
		AddConfigParam("kube_tag_prefix", fmt.Sprintf("%s.var.log.containers.", pipeline.Name)).
		AddConfigParam("annotations", fluentBitBool(keepAnnotations)).
		AddConfigParam("labels", fluentBitBool(keepLabels)).
		AddConfigParam("buffer_size", "1MB").
		AddConfigParam("keep_log", fluentBitBool(keepOriginalBody)).
		Build()
}

func fluentBitBool(b bool) string {
	if b {
		return "on"
	}

	return "off"
}
