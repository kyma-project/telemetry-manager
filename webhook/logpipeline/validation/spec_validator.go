package validation

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
	pipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/pipelines"
)

var (
	forbiddenFilters             = []string{"kubernetes", "rewrite_tag"}
	validHostNamePattern         = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)
	ErrInvalidPipelineDefinition = errors.New("invalid log pipeline definition")
)

func ValidateSpec(lp *telemetryv1alpha1.LogPipeline) error {
	if err := validateOutput(lp); err != nil {
		return err
	}

	if err := validateFilters(lp); err != nil {
		return err
	}

	return validateInput(lp)
}

func validateOutput(lp *telemetryv1alpha1.LogPipeline) error {
	output := lp.Spec.Output
	if err := checkSingleOutputPlugin(output); err != nil {
		return err
	}

	if pipelineutils.IsHTTPDefined(&output) {
		if err := validateHTTPOutput(output.HTTP); err != nil {
			return err
		}
	}

	return validateCustomOutput(output.Custom)
}

func checkSingleOutputPlugin(output telemetryv1alpha1.LogPipelineOutput) error {
	if !pipelineutils.IsAnyDefined(&output) {
		return fmt.Errorf("no output plugin is defined, you must define one output plugin")
	}

	if !pipelineutils.IsSingleDefined(&output) {
		return fmt.Errorf("multiple output plugins are defined, you must define only one output plugin")
	}

	return nil
}

func validateHTTPOutput(httpOutput *telemetryv1alpha1.LogPipelineHTTPOutput) error {
	isValidHostname := validHostname(httpOutput.Host.Value)

	if httpOutput.Host.Value != "" && !isValidHostname {
		return fmt.Errorf("invalid hostname '%s'", httpOutput.Host.Value)
	}

	if httpOutput.URI != "" && !strings.HasPrefix(httpOutput.URI, "/") {
		return fmt.Errorf("uri must start with /")
	}

	if secretRefAndValueIsPresent(httpOutput.Host) {
		return fmt.Errorf("http output host must have either a value or secret key reference")
	}

	if secretRefAndValueIsPresent(httpOutput.User) {
		return fmt.Errorf("http output user must have either a value or secret key reference")
	}

	if secretRefAndValueIsPresent(httpOutput.Password) {
		return fmt.Errorf("http output password must have either a value or secret key reference")
	}

	return nil
}

func validHostname(host string) bool {
	host = strings.Trim(host, " ")
	return validHostNamePattern.MatchString(host)
}

func validateCustomOutput(content string) error {
	if content == "" {
		return nil
	}

	section, err := config.ParseCustomSection(content)
	if err != nil {
		return err
	}

	if !section.ContainsKey("name") {
		return fmt.Errorf("configuration section must have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	if section.ContainsKey("match") {
		return fmt.Errorf("output plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	if section.ContainsKey("storage.total_limit_size") {
		return fmt.Errorf("output plugin '%s' contains forbidden configuration key 'storage.total_limit_size'", pluginName)
	}

	return nil
}

func secretRefAndValueIsPresent(v telemetryv1alpha1.ValueType) bool {
	return v.Value != "" && v.ValueFrom != nil
}

func validateFilters(lp *telemetryv1alpha1.LogPipeline) error {
	// TODO[k15r]: validate Filters in OTLP mode
	for _, filterPlugin := range lp.Spec.Filters {
		if err := validateCustomFilter(filterPlugin.Custom); err != nil {
			return err
		}
	}

	return nil
}

func validateCustomFilter(content string) error {
	if content == "" {
		return nil
	}

	section, err := config.ParseCustomSection(content)
	if err != nil {
		return err
	}

	if !section.ContainsKey("name") {
		return fmt.Errorf("configuration section must have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	for _, forbiddenFilter := range forbiddenFilters {
		if strings.EqualFold(pluginName, forbiddenFilter) {
			return fmt.Errorf("filter plugin '%s' is forbidden. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("filter plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	return nil
}

func validateInput(lp *telemetryv1alpha1.LogPipeline) error {
	input := lp.Spec.Input
	if !pipelineutils.IsInputValid(&input) {
		return nil
	}

	// Pipeline Mode is OTel
	if lp.Spec.Output.OTLP != nil {
		return validateApplication(lp)
	}

	// Pipeline Mode is FluentBit
	if lp.Spec.Input.OTLP != nil {
		return fmt.Errorf("%w: cannot use OTLP input for pipeline in FluentBit mode", ErrInvalidPipelineDefinition)
	}

	return validateApplication(lp)
}

func validateApplication(lp *telemetryv1alpha1.LogPipeline) error {
	application := lp.Spec.Input.Application
	if application == nil {
		return nil
	}

	containers := application.Containers

	if len(containers.Include) > 0 && len(containers.Exclude) > 0 {
		return fmt.Errorf("%w: Cannot define both 'input.application.containers.include' and 'input.application.containers.exclude'", ErrInvalidPipelineDefinition)
	}

	namespaces := application.Namespaces

	if (len(namespaces.Include) > 0 && len(namespaces.Exclude) > 0) ||
		(len(namespaces.Include) > 0 && namespaces.System) ||
		(len(namespaces.Exclude) > 0 && namespaces.System) {
		return fmt.Errorf("%w: Can only define one 'input.application.namespaces' selector - either 'include', 'exclude', or 'system'", ErrInvalidPipelineDefinition)
	}

	return nil
}
