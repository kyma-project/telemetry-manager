package fluentbit

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

var forbiddenFilters = []string{"kubernetes", "rewrite_tag"}

type EndpointValidator interface {
	Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, protocol string) error
}

type TLSCertValidator interface {
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

type SecretRefValidator interface {
	ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
}

type PipelineLock interface {
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
	IsLockHolder(ctx context.Context, owner metav1.Object) error
}

type Validator struct {
	EndpointValidator  EndpointValidator
	TLSCertValidator   TLSCertValidator
	SecretRefValidator SecretRefValidator
	PipelineLock       PipelineLock
}

func (v *Validator) Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error {
	if err := v.PipelineLock.TryAcquireLock(ctx, pipeline); err != nil {
		return err
	}

	if err := v.SecretRefValidator.ValidateLogPipeline(ctx, pipeline); err != nil {
		return err
	}

	if pipeline.Spec.Output.HTTP != nil {
		if err := v.EndpointValidator.Validate(ctx, &pipeline.Spec.Output.HTTP.Host, endpoint.FluentdProtocolHTTP); err != nil {
			return err
		}
	}

	if tlsValidationRequired(pipeline) {
		tlsConfig := tlscert.TLSBundle{
			Cert: pipeline.Spec.Output.HTTP.TLS.Cert,
			Key:  pipeline.Spec.Output.HTTP.TLS.Key,
			CA:   pipeline.Spec.Output.HTTP.TLS.CA,
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

	return nil
}

func tlsValidationRequired(pipeline *telemetryv1alpha1.LogPipeline) bool {
	http := pipeline.Spec.Output.HTTP
	if http == nil {
		return false
	}

	return http.TLS.Cert != nil || http.TLS.Key != nil || http.TLS.CA != nil
}

func validateFileNames(logpipeline *telemetryv1alpha1.LogPipeline) error {
	files := logpipeline.Spec.Files
	uniqFileMap := make(map[string]bool)

	for _, f := range files {
		uniqFileMap[f.Name] = true
	}

	if len(uniqFileMap) != len(files) {
		return fmt.Errorf("duplicate file names detected please review your pipeline")
	}

	return nil
}

func validateFilters(lp *telemetryv1alpha1.LogPipeline) error {
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
