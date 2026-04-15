package common

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

// =============================================================================
// OAuth2 EXTENSION CONFIG BUILDER
// =============================================================================

type OAuth2ExtensionConfigBuilder struct {
	reader        client.Reader
	oauth2Options *telemetryv1beta1.OAuth2Options
	pipelineRef   PipelineRef
}

func NewOAuth2ExtensionConfigBuilder(reader client.Reader, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineRef PipelineRef) *OAuth2ExtensionConfigBuilder {
	return &OAuth2ExtensionConfigBuilder{
		reader:        reader,
		oauth2Options: oauth2Options,
		pipelineRef:   pipelineRef,
	}
}

func (cb *OAuth2ExtensionConfigBuilder) OAuth2Extension(ctx context.Context) (*OAuth2ExtensionConfig, EnvVars, error) {
	envVars, err := makeOAuth2ExtensionEnvVars(ctx, cb.reader, cb.oauth2Options, cb.pipelineRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %w", err)
	}

	extension := oauth2Extension(cb.oauth2Options, cb.pipelineRef)

	return extension, envVars, nil
}

func oauth2Extension(oauth2Options *telemetryv1beta1.OAuth2Options, pipelineRef PipelineRef) *OAuth2ExtensionConfig {
	return &OAuth2ExtensionConfig{
		TokenURL:     fmt.Sprintf("${%s}", formatEnvVarKey(oauth2TokenURLVariablePrefix, pipelineRef)),
		ClientID:     fmt.Sprintf("${%s}", formatEnvVarKey(oauth2ClientIDVariablePrefix, pipelineRef)),
		ClientSecret: fmt.Sprintf("${%s}", formatEnvVarKey(oauth2ClientSecretVariablePrefix, pipelineRef)),
		Scopes:       oauth2Options.Scopes,
		Params:       oauth2Options.Params,
	}
}
