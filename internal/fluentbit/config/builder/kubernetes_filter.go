package builder

import (
	"fmt"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func createKubernetesFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	sb := NewFilterSectionBuilder().
		AddConfigParam("name", "kubernetes").
		AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name)).
		AddConfigParam("merge_log", "on").
		AddConfigParam("k8s-logging.parser", "on").
		AddConfigParam("k8s-logging.exclude", "off").
		AddConfigParam("kube_tag_prefix", fmt.Sprintf("%s.var.log.containers.", pipeline.Name)).
		AddConfigParam("annotations", fmt.Sprintf("%v", fluentBitFlag(pipeline.Spec.Input.Application.KeepAnnotations))).
		AddConfigParam("labels", fmt.Sprintf("%v", fluentBitFlag(!pipeline.Spec.Input.Application.DropLabels))).
		AddConfigParam("buffer_size", "1MB")
	if dropLogBody(pipeline) {
		sb.AddConfigParam("keep_log", "off")
	}
	return sb.Build()
}

func fluentBitFlag(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func dropLogBody(pipeline *telemetryv1alpha1.LogPipeline) bool {
	if pipeline.Spec.Output.HTTP != nil && !pipeline.Spec.Output.HTTP.KeepBody {
		return true
	}
	return false
}
