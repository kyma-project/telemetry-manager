package builder

import (
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

func getEmitterPostfixByOutput(output *telemetryv1alpha1.LogPipelineOutput) string {
	if logpipelineutils.IsHTTPDefined(output) {
		return "http"
	}

	if !logpipelineutils.IsCustomDefined(output) {
		return ""
	}

	customOutputParams := parseMultiline(output.Custom)
	postfix := customOutputParams.GetByKey("name")

	if postfix == nil {
		return ""
	}

	return postfix.Value
}

func createRewriteTagFilter(logPipeline *telemetryv1alpha1.LogPipeline, defaults PipelineDefaults) string {
	emitterName := logPipeline.Name
	output := &logPipeline.Spec.Output
	emitterPostfix := getEmitterPostfixByOutput(output)

	if emitterPostfix != "" {
		emitterName += ("-" + emitterPostfix)
	}

	var sectionBuilder = NewFilterSectionBuilder().
		AddConfigParam("Name", "rewrite_tag").
		AddConfigParam("Match", fmt.Sprintf("%s.*", defaults.InputTag)).
		AddConfigParam("Emitter_Name", emitterName).
		AddConfigParam("Emitter_Storage.type", defaults.StorageType).
		AddConfigParam("Emitter_Mem_Buf_Limit", defaults.MemoryBufferLimit)

	var containers telemetryv1alpha1.LogPipelineContainerSelector
	if logPipeline.Spec.Input.Application != nil {
		containers = logPipeline.Spec.Input.Application.Containers
	}

	if len(containers.Include) > 0 {
		return sectionBuilder.
			AddConfigParam("Rule", fmt.Sprintf("$kubernetes['container_name'] \"^(%s)$\" %s.$TAG true",
				strings.Join(containers.Include, "|"), logPipeline.Name)).
			Build()
	}

	if len(containers.Exclude) > 0 {
		return sectionBuilder.
			AddConfigParam("Rule", fmt.Sprintf("$kubernetes['container_name'] \"^(?!%s$).*\" %s.$TAG true",
				strings.Join(containers.Exclude, "$|"), logPipeline.Name)).
			Build()
	}

	return sectionBuilder.
		AddConfigParam("Rule", fmt.Sprintf("$log \"^.*$\" %s.$TAG true", logPipeline.Name)).
		Build()
}
