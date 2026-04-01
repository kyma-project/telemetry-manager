package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestOAuth2ExtensionID(t *testing.T) {
	require.Equal(t, "oauth2client/tracepipeline-test", OAuth2ExtensionID(PipelineRef{Name: "test", Type: SignalTypeTrace, UseTypePrefix: true}))
}

func TestOAuth2ExtensionIDNoPrefix(t *testing.T) {
	require.Equal(t, "oauth2client/test", OAuth2ExtensionID(PipelineRef{Name: "test"}))
}

func TestMakeExtensionConfig(t *testing.T) {
	oauth2Options := &telemetryv1beta1.OAuth2Options{
		TokenURL:     telemetryv1beta1.ValueType{Value: "token-url"},
		ClientID:     telemetryv1beta1.ValueType{Value: "client-id"},
		ClientSecret: telemetryv1beta1.ValueType{Value: "client-secret"},
	}

	ref := PipelineRef{Name: "test", Type: SignalTypeTrace, UseTypePrefix: true}
	cb := NewOAuth2ExtensionConfigBuilder(fake.NewClientBuilder().Build(), oauth2Options, ref)
	oauth2ExtensionConfig, envVars, err := cb.OAuth2Extension(t.Context())
	require.NoError(t, err)
	require.NotNil(t, envVars)

	require.NotNil(t, envVars["OAUTH2_TOKEN_URL_TRACEPIPELINE_TEST"])
	require.Equal(t, envVars["OAUTH2_TOKEN_URL_TRACEPIPELINE_TEST"], []byte("token-url"))

	require.NotNil(t, envVars["OAUTH2_CLIENT_ID_TRACEPIPELINE_TEST"])
	require.Equal(t, envVars["OAUTH2_CLIENT_ID_TRACEPIPELINE_TEST"], []byte("client-id"))

	require.NotNil(t, envVars["OAUTH2_CLIENT_SECRET_TRACEPIPELINE_TEST"])
	require.Equal(t, envVars["OAUTH2_CLIENT_SECRET_TRACEPIPELINE_TEST"], []byte("client-secret"))

	require.Equal(t, "${OAUTH2_TOKEN_URL_TRACEPIPELINE_TEST}", oauth2ExtensionConfig.TokenURL)
	require.Equal(t, "${OAUTH2_CLIENT_ID_TRACEPIPELINE_TEST}", oauth2ExtensionConfig.ClientID)
	require.Equal(t, "${OAUTH2_CLIENT_SECRET_TRACEPIPELINE_TEST}", oauth2ExtensionConfig.ClientSecret)
}
