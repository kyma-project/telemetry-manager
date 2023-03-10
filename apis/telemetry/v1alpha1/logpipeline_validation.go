package v1alpha1

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

func (lp *LogPipeline) Validate() error {
	if err := lp.validateInput(&lp.Spec.Input); err != nil {
		return err
	}

	if err := lp.validateOutput(&lp.Spec.Output); err != nil {
		return err
	}

	return nil
}

func (lp *LogPipeline) validateInput(logPipelineInput *Input) error {
	if logPipelineInput == nil {
		return nil
	}

	var containers = logPipelineInput.Application.Containers
	if len(containers.Include) > 0 && len(containers.Exclude) > 0 {
		return fmt.Errorf("invalid log pipeline definition: can not define both 'input.application.containers.include' and 'input.application.containers.exclude'")
	}

	var namespaces = logPipelineInput.Application.Namespaces
	if (len(namespaces.Include) > 0 && len(namespaces.Exclude) > 0) ||
		(len(namespaces.Include) > 0 && namespaces.System) ||
		(len(namespaces.Exclude) > 0 && namespaces.System) {
		return fmt.Errorf("invalid log pipeline definition: can only define one of 'input.application.namespaces' selectors: 'include', 'exclude', 'system'")
	}

	return nil
}

func (lp *LogPipeline) validateOutput(output *Output) error {
	if err := checkSingleOutputPlugin(*output); err != nil {
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
