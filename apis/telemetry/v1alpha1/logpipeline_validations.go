package v1alpha1

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
	"net/url"
	"regexp"
	"strings"
)

func (l *LogPipeline) Validate(vc *LogPipelineValidationConfig) error {
	if err := l.validateOutput(vc.DeniedOutPutPlugins); err != nil {
		return err
	}
	if err := l.validateFilters(vc.DeniedFilterPlugins); err != nil {
		return err
	}
	if err := l.validateInput(); err != nil {
		return err
	}
	return nil
}

func (l *LogPipeline) validateOutput(deniedOutputPlugins []string) error {
	output := l.Spec.Output
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

	if err := validateCustomOutput(deniedOutputPlugins, output.Custom); err != nil {
		return err
	}

	return nil
}

func checkSingleOutputPlugin(output Output) error {
	if !output.IsAnyDefined() {
		return fmt.Errorf("no output is defined, you must define one output")
	}
	if !output.IsSingleDefined() {
		return fmt.Errorf("multiple output plugins are defined, you must define only one output")
	}
	return nil
}

func validateLokiOutput(lokiOutput *LokiOutput) error {
	if lokiOutput.URL.Value != "" && !validURL(lokiOutput.URL.Value) {
		return fmt.Errorf("invalid hostname '%s'", lokiOutput.URL.Value)
	}
	if !lokiOutput.URL.IsDefined() && (len(lokiOutput.Labels) != 0 || len(lokiOutput.RemoveKeys) != 0) {
		return fmt.Errorf("loki output needs to have a URL configured")
	}
	if secretRefAndValueIsPresent(lokiOutput.URL) {
		return fmt.Errorf("loki output URL needs to have either value or secret key reference")
	}
	return nil

}

func validateHTTPOutput(httpOutput *HTTPOutput) error {
	if httpOutput.Host.Value != "" && !validHostname(httpOutput.Host.Value) {
		return fmt.Errorf("invalid hostname '%s'", httpOutput.Host.Value)
	}
	if httpOutput.URI != "" && !strings.HasPrefix(httpOutput.URI, "/") {
		return fmt.Errorf("uri has to start with /")
	}
	if !httpOutput.Host.IsDefined() && (httpOutput.User.IsDefined() || httpOutput.Password.IsDefined() || httpOutput.URI != "" || httpOutput.Port != "" || httpOutput.Compress != "" || httpOutput.TLSConfig.Disabled || httpOutput.TLSConfig.SkipCertificateValidation) {
		return fmt.Errorf("http output needs to have a host configured")
	}
	if secretRefAndValueIsPresent(httpOutput.Host) {
		return fmt.Errorf("http output host needs to have either value or secret key reference")
	}
	if secretRefAndValueIsPresent(httpOutput.User) {
		return fmt.Errorf("http output user needs to have either value or secret key reference")
	}
	if secretRefAndValueIsPresent(httpOutput.Password) {
		return fmt.Errorf("http output password needs to have either value or secret key reference")
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

func validHostname(host string) bool {
	host = strings.Trim(host, " ")
	re, _ := regexp.Compile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)
	return re.MatchString(host)
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
		return fmt.Errorf("configuration section does not have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	for _, deniedPlugin := range deniedOutputPlugin {
		if strings.EqualFold(pluginName, deniedPlugin) {
			return fmt.Errorf("output plugin '%s' is forbidden. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	if section.ContainsKey("storage.total_limit_size") {
		return fmt.Errorf("plugin '%s' contains forbidden configuration key 'storage.total_limit_size'", pluginName)
	}

	return nil
}

func secretRefAndValueIsPresent(v ValueType) bool {
	return v.Value != "" && v.ValueFrom != nil
}

func (l *LogPipeline) validateFilters(deniedFilterPlugins []string) error {
	for _, filterPlugin := range l.Spec.Filters {
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
		return fmt.Errorf("configuration section does not have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	for _, deniedPlugin := range deniedFilterPlugins {
		if strings.EqualFold(pluginName, deniedPlugin) {
			return fmt.Errorf("filter plugin '%s' is forbidden. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	return nil
}

func (l *LogPipeline) validateInput() error {
	input := l.Spec.Input
	if !input.IsDefined() {
		return nil
	}

	var containers = input.Application.Containers
	if len(containers.Include) > 0 && len(containers.Exclude) > 0 {
		return fmt.Errorf("invalid log pipeline definition: can not define both 'input.application.containers.include' and 'input.application.containers.exclude'")
	}

	var namespaces = input.Application.Namespaces
	if (len(namespaces.Include) > 0 && len(namespaces.Exclude) > 0) ||
		(len(namespaces.Include) > 0 && namespaces.System) ||
		(len(namespaces.Exclude) > 0 && namespaces.System) {
		return fmt.Errorf("invalid log pipeline definition: can only define one of 'input.application.namespaces' selectors: 'include', 'exclude', 'system'")
	}

	return nil
}
