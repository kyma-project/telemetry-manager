package builder

import (
	"errors"
	"fmt"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
)

var (
	ErrUndefinedOutputPlugin = errors.New("output plugin not defined")
)

type PipelineDefaults struct {
	InputTag          string
	MemoryBufferLimit string
	StorageType       string
	FsBufferLimit     string
}

// FluentBit builder configuration
type BuilderConfig struct {
	PipelineDefaults
	CollectAgentLogs bool
}

// BuildFluentBitConfig merges Fluent Bit filters and outputs to a single Fluent Bit configuration.
func BuildFluentBitConfig(pipeline *telemetryv1alpha1.LogPipeline, config BuilderConfig) (string, error) {
	err := validateOutput(pipeline)
	if err != nil {
		return "", err
	}

	err = validateCustomSections(pipeline)
	if err != nil {
		return "", err
	}

	includePath := createIncludePath(pipeline)
	excludePath := createExcludePath(pipeline, config.CollectAgentLogs)

	var sb strings.Builder

	sb.WriteString(createInputSection(pipeline, includePath, excludePath))
	// skip if the filter is a multiline filter, multiline filter should be first filter in the pipeline filter chain
	// see for more details https://docs.fluentbit.io/manual/pipeline/filters/multiline-stacktrace
	sb.WriteString(createCustomFilters(pipeline, multilineFilter))
	sb.WriteString(createRecordModifierFilter(pipeline))
	sb.WriteString(createKubernetesFilter(pipeline))
	sb.WriteString(createCustomFilters(pipeline, nonMultilineFilter))
	sb.WriteString(createLuaDedotFilter(pipeline))
	sb.WriteString(createOutputSection(pipeline, config.PipelineDefaults))

	return sb.String(), nil
}

func createRecordModifierFilter(pipeline *telemetryv1alpha1.LogPipeline) string {
	return NewFilterSectionBuilder().
		AddConfigParam("name", "record_modifier").
		AddConfigParam("match", fmt.Sprintf("%s.*", pipeline.Name)).
		AddConfigParam("record", "cluster_identifier ${KUBERNETES_SERVICE_HOST}").
		Build()
}

func createLuaDedotFilter(logPipeline *telemetryv1alpha1.LogPipeline) string {
	output := logPipeline.Spec.Output
	if !output.IsHTTPDefined() || !output.HTTP.Dedot {
		return ""
	}

	return NewFilterSectionBuilder().
		AddConfigParam("name", "lua").
		AddConfigParam("match", fmt.Sprintf("%s.*", logPipeline.Name)).
		AddConfigParam("script", "/fluent-bit/scripts/filter-script.lua").
		AddConfigParam("call", "kubernetes_map_keys").
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
	if !pipeline.Spec.Output.IsAnyDefined() {
		return ErrUndefinedOutputPlugin
	}

	return nil
}
