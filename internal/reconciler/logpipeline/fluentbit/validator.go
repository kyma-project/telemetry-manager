package fluentbit

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

var forbiddenFilters = []string{"kubernetes", "rewrite_tag"}
var validHostNamePattern = regexp.MustCompile(`^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\-]*[A-Za-z0-9])$`)

type Validator struct {
	EndpointValidator  EndpointValidator
	TLSCertValidator   TLSCertValidator
	SecretRefValidator SecretRefValidator
	PipelineLock       PipelineLock
}

// ValidatorOption configures the Validator during initialization.
type ValidatorOption func(*Validator)

// WithEndpointValidator sets the endpoint validator for the Validator.
func WithEndpointValidator(validator EndpointValidator) ValidatorOption {
	return func(v *Validator) {
		v.EndpointValidator = validator
	}
}

// WithTLSCertValidator sets the TLS certificate validator for the Validator.
func WithTLSCertValidator(validator TLSCertValidator) ValidatorOption {
	return func(v *Validator) {
		v.TLSCertValidator = validator
	}
}

// WithSecretRefValidator sets the secret reference validator for the Validator.
func WithSecretRefValidator(validator SecretRefValidator) ValidatorOption {
	return func(v *Validator) {
		v.SecretRefValidator = validator
	}
}

// WithValidatorPipelineLock sets the pipeline lock for the Validator.
func WithValidatorPipelineLock(lock PipelineLock) ValidatorOption {
	return func(v *Validator) {
		v.PipelineLock = lock
	}
}

// NewValidator creates a new Validator with the provided options.
func NewValidator(opts ...ValidatorOption) *Validator {
	v := &Validator{}

	for _, opt := range opts {
		opt(v)
	}

	return v
}

func (v *Validator) Validate(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error {
	if err := v.PipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		return err
	}

	if err := v.SecretRefValidator.ValidateLogPipeline(ctx, pipeline); err != nil {
		return err
	}

	if pipeline.Spec.Output.FluentBitHTTP != nil {
		if err := v.EndpointValidator.Validate(ctx, endpoint.EndpointValidationParams{
			Endpoint: &pipeline.Spec.Output.FluentBitHTTP.Host,
			Protocol: endpoint.FluentdProtocolHTTP,
		}); err != nil {
			return err
		}
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSValidationParams{
			Cert: pipeline.Spec.Output.FluentBitHTTP.TLS.Cert,
			Key:  pipeline.Spec.Output.FluentBitHTTP.TLS.Key,
			CA:   pipeline.Spec.Output.FluentBitHTTP.TLS.CA,
		}

		if err := v.TLSCertValidator.Validate(ctx, tlsConfig); err != nil {
			return err
		}
	}

	if err := validateFileNames(pipeline); err != nil {
		return err
	}

	if err := validateFilters(pipeline); err != nil {
		return err
	}

	if err := validateCustomOutput(pipeline.Spec.Output.FluentBitCustom); err != nil {
		return err
	}

	httpOutput := pipeline.Spec.Output.FluentBitHTTP
	if httpOutput != nil && httpOutput.Host.Value != "" && !isValidHostname(httpOutput.Host.Value) {
		return fmt.Errorf("invalid hostname '%s'", httpOutput.Host.Value)
	}

	return nil
}

func tlsValidationRequired(pipeline *telemetryv1beta1.LogPipeline) bool {
	http := pipeline.Spec.Output.FluentBitHTTP
	if http == nil {
		return false
	}

	return http.TLS.Cert != nil || http.TLS.Key != nil || http.TLS.CA != nil
}

func isValidHostname(host string) bool {
	host = strings.Trim(host, " ")
	return validHostNamePattern.MatchString(host)
}

func validateFileNames(logpipeline *telemetryv1beta1.LogPipeline) error {
	files := logpipeline.Spec.FluentBitFiles
	uniqFileMap := make(map[string]bool)

	for _, f := range files {
		uniqFileMap[f.Name] = true
	}

	if len(uniqFileMap) != len(files) {
		return fmt.Errorf("duplicate file names detected please review your pipeline")
	}

	return nil
}

func validateFilters(lp *telemetryv1beta1.LogPipeline) error {
	for _, filterPlugin := range lp.Spec.FluentBitFilters {
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
		return fmt.Errorf("custom filter configuration must have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	for _, forbiddenFilter := range forbiddenFilters {
		if strings.EqualFold(pluginName, forbiddenFilter) {
			return fmt.Errorf("custom filter plugin '%s' is not supported. ", pluginName)
		}
	}

	if section.ContainsKey("match") {
		return fmt.Errorf("custom filter plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	return nil
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
		return fmt.Errorf("custom output configuration section must have name attribute")
	}

	pluginName := section.GetByKey("name").Value

	if section.ContainsKey("match") {
		return fmt.Errorf("custom output plugin '%s' contains match condition. Match conditions are forbidden", pluginName)
	}

	if section.ContainsKey("storage.total_limit_size") {
		return fmt.Errorf("custom output plugin '%s' contains forbidden configuration key 'storage.total_limit_size'", pluginName)
	}

	return nil
}
