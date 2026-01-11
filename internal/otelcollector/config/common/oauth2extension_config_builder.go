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
	pipelineName  string
	signalType    string
}

func NewOAuth2ExtensionConfigBuilder(reader client.Reader, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineName string, signalType string) *OAuth2ExtensionConfigBuilder {
	return &OAuth2ExtensionConfigBuilder{
		reader:        reader,
		oauth2Options: oauth2Options,
		pipelineName:  pipelineName,
		signalType:    signalType,
	}
}

func (cb *OAuth2ExtensionConfigBuilder) OAuth2ExtensionConfig(ctx context.Context) (*OAuth2Extension, EnvVars, error) {
	envVars, err := makeOAuth2ExtensionEnvVars(ctx, cb.reader, cb.oauth2Options, cb.pipelineName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make env vars: %w", err)
	}

	extensionsConfig := makeExtensionConfig(cb.oauth2Options, cb.pipelineName)

	return extensionsConfig, envVars, nil
}

func makeExtensionConfig(oauth2Options *telemetryv1beta1.OAuth2Options, pipelineName string) *OAuth2Extension {
	return &OAuth2Extension{
		TokenURL:     fmt.Sprintf("${%s}", formatEnvVarKey(oauth2TokenURLVariablePrefix, pipelineName)),
		ClientID:     fmt.Sprintf("${%s}", formatEnvVarKey(oauth2ClientIDVariablePrefix, pipelineName)),
		ClientSecret: fmt.Sprintf("${%s}", formatEnvVarKey(oauth2ClientSecretVariablePrefix, pipelineName)),
		Scopes:       oauth2Options.Scopes,
		Params:       oauth2Options.Params,
	}
}

func OAuth2ExtensionID(pipelineName string) string {
	return fmt.Sprintf(ComponentIDOAuth2Extension, pipelineName)
}
