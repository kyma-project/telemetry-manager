package builder

import (
	"errors"
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

var (
	errInvalidPipelineDefinition = errors.New("invalid pipeline definition")
)

// buildFluentBitSectionsConfig merges Fluent Bit filters and outputs to a single Fluent Bit configuration.
func buildFluentBitSectionsConfig(pipeline *telemetryv1alpha1.LogPipeline, config builderConfig) (string, error) {
	pm := logpipelineutils.PipelineMode(pipeline)
	if pm != logpipelineutils.FluentBit {
		return "", fmt.Errorf("%w: unsupported pipeline mode: %s", errInvalidPipelineDefinition, pm.String())
	}

	err := validateOutput(pipeline)
	if err != nil {
		return "", err
	}

	err = validateInput(pipeline)
	if err != nil {
		return "", err
	}

	err = validateCustomSections(pipeline)
	if err != nil {
		return "", err
	}

	includePath := createIncludePath(pipeline)
	excludePath := createExcludePath(pipeline, config.collectAgentLogs)

	var sb strings.Builder

	sb.WriteString(createInputSection(pipeline, includePath, excludePath))
	// skip if the filter is a multiline filter, multiline filter should be first filter in the pipeline filter chain
	// see for more details https://docs.fluentbit.io/manual/pipeline/filters/multiline-stacktrace
	sb.WriteString(createCustomFilters(pipeline, multilineFilter))
	sb.WriteString(createRecordModifierFilter(pipeline))
	sb.WriteString(createKubernetesFilter(pipeline))
	sb.WriteString(createTimestampModifyFilter(pipeline))
	sb.WriteString(createCustomFilters(pipeline, nonMultilineFilter))
	sb.WriteString(createLuaFilter(pipeline))
	sb.WriteString(createOutputSection(pipeline, config.pipelineDefaults))

	return sb.String(), nil
}

func createRecordModifierFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	return NewFilterSectionBuilder().
		AddConfigParam("name", "record_modifier").
		AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name)).
		AddConfigParam("record", "cluster_identifier ${KUBERNETES_SERVICE_HOST}").
		Build()
}

func createLuaFilter(logPipeline *telemetryv1alpha1.LogPipeline) string {
	output := logPipeline.Spec.Output
	if !logpipelineutils.IsHTTPDefined(&output) {
		return ""
	}

	call := "enrich_app_name"
	if output.HTTP.Dedot {
		call = "dedot_and_enrich_app_name"
	}

	return NewFilterSectionBuilder().
		AddConfigParam("name", "lua").
		AddConfigParam("match", fmt.Sprintf("%s.*", logPipeline.Name)).
		AddConfigParam("script", "/fluent-bit/scripts/filter-script.lua").
		AddConfigParam("call", call).
		Build()
}

func validateCustomSections(pipeline *telemetryv1alpha1.LogPipeline) error {
	customOutput := pipeline.Spec.Output.Custom
	if customOutput != "" {
		_, err := config.ParseCustomSection(customOutput)
		if err != nil {
			return err
		}
	}

	for _, filter := range pipeline.Spec.Filters {
		_, err := config.ParseCustomSection(filter.Custom)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateOutput(pipeline *telemetryv1alpha1.LogPipeline) error {
	if !logpipelineutils.IsAnyDefined(&pipeline.Spec.Output) {
		return fmt.Errorf("%w: No output plugin defined", errInvalidPipelineDefinition)
	}

	return nil
}

func validateInput(pipeline *telemetryv1alpha1.LogPipeline) error {
	if pipeline.Spec.Input.OTLP != nil {
		return fmt.Errorf("%w: cannot use OTLP input for pipeline in FluentBit mode", errInvalidPipelineDefinition)
	}

	if pipeline.Spec.Input.Application == nil {
		return nil
	}

	return nil
}

func createTimestampModifyFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	output := pipeline.Spec.Output
	if !logpipelineutils.IsHTTPDefined(&output) {
		return ""
	}

	return NewFilterSectionBuilder().
		AddConfigParam("name", "modify").
		AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name)).
		AddConfigParam("copy", "time @timestamp").
		Build()
}
