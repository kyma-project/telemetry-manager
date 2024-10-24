package builder

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func createKubernetesFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	appInput := &telemetryv1alpha1.LogPipelineApplicationInput{}
	if pipeline.Spec.Input.Application != nil {
		appInput = pipeline.Spec.Input.Application
	}

	keepAnnotations := appInput.KeepAnnotations
	keepLabels := !appInput.DropLabels

	keepOriginalBody := true
	if appInput.KeepOriginalBody != nil {
		keepOriginalBody = *appInput.KeepOriginalBody
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
