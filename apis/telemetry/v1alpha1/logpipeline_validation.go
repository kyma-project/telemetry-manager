package v1alpha1

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
)

func (lp *LogPipeline) Validate(vc *LogPipelineValidationConfig) error {
	if err := lp.validateOutput(vc.DeniedOutPutPlugins); err != nil {
		return err
	}
	if err := lp.validateFilters(vc.DeniedFilterPlugins); err != nil {
		return err
	}
	return lp.validateInput()
}

func (lp *LogPipeline) validateOutput(deniedOutputPlugins []string) error {
	output := lp.Spec.Output
	if err := checkSingleOutputPlugin(output); err != nil {
		return err
	}

	if output.IsHTTPDefined() {
		if err := validateHTTPOutput(output.HTTP); err != nil {
			return err
		}
	}

	if output.IsLokiDefined() {
		if err := validateLokiOutput(output.Loki); err != nil {
			return err
		}
	}

	return validateCustomOutput(deniedOutputPlugins, output.Custom)
}

func checkSingleOutputPlugin(output Output) error {
	if !output.IsAnyDefined() {
		return fmt.Errorf("no output plugin is defined, you must define one output plugin")
	}
	if !output.IsSingleDefined() {
		return fmt.Errorf("multiple output plugins are defined, you must define only one output plugin")
	}
	return nil
}

func validateLokiOutput(lokiOutput *LokiOutput) error {
	if lokiOutput.URL.Value != "" && !validURL(lokiOutput.URL.Value) {
		return fmt.Errorf("invalid hostname '%s'", lokiOutput.URL.Value)
	}
	if !lokiOutput.URL.IsDefined() && (len(lokiOutput.Labels) != 0 || len(lokiOutput.RemoveKeys) != 0) {
		return fmt.Errorf("loki output must have a URL configured")
	}
	if secretRefAndValueIsPresent(lokiOutput.URL) {
		return fmt.Errorf("loki output URL must have either a value or secret key reference")
	}
	return nil

}

func validateHTTPOutput(httpOutput *HTTPOutput) error {
	isValidHostname, err := validHostname(httpOutput.Host.Value)

	if err != nil {
		return fmt.Errorf("error validating hostname: %w", err)
	}

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

func validURL(host string) bool {
	host = strings.Trim(host, " ")

	_, err := url.ParseRequestURI(host)
	if err != nil {
		return false
	}

	u, err := url.Parse(host)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return true
}

func validHostname(host string) (bool, error) {
	host = strings.Trim(host, " ")
	re, err := regexp.Compile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)
	return re.MatchString(host), err
}

func validateCustomOutput(deniedOutputPlugin []string, content string) error {
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

	for _, deniedPlugin := range deniedOutputPlugin {
		if strings.EqualFold(pluginName, deniedPlugin) {
			return fmt.Errorf("output plugin '%s' is forbidden. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("output plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	if section.ContainsKey("storage.total_limit_size") {
		return fmt.Errorf("output plugin '%s' contains forbidden configuration key 'storage.total_limit_size'", pluginName)
	}

	return nil
}

func secretRefAndValueIsPresent(v ValueType) bool {
	return v.Value != "" && v.ValueFrom != nil
}

func (lp *LogPipeline) validateFilters(deniedFilterPlugins []string) error {
	for _, filterPlugin := range lp.Spec.Filters {
		if err := validateCustomFilter(filterPlugin.Custom, deniedFilterPlugins); err != nil {
			return err
		}
	}
	return nil
}

func validateCustomFilter(content string, deniedFilterPlugins []string) error {
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

	for _, deniedPlugin := range deniedFilterPlugins {
		if strings.EqualFold(pluginName, deniedPlugin) {
			return fmt.Errorf("filter plugin '%s' is forbidden. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("filter plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	return nil
}

func (lp *LogPipeline) validateInput() error {
	input := lp.Spec.Input
	if !input.IsDefined() {
		return nil
	}

	var containers = input.Application.Containers
	if len(containers.Include) > 0 && len(containers.Exclude) > 0 {
		return fmt.Errorf("invalid log pipeline definition: Cannot define both 'input.application.containers.include' and 'input.application.containers.exclude'")
	}

	var namespaces = input.Application.Namespaces
	if (len(namespaces.Include) > 0 && len(namespaces.Exclude) > 0) ||
		(len(namespaces.Include) > 0 && namespaces.System) ||
		(len(namespaces.Exclude) > 0 && namespaces.System) {
		return fmt.Errorf("invalid log pipeline definition: Can only define one 'input.application.namespaces' selector - either 'include', 'exclude', or 'system'")
	}

	return nil
}
