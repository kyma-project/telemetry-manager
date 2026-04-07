package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestOAuth2ExtensionID(t *testing.T) {
	ref := TracePipelineRef(&telemetryv1beta1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
	require.Equal(t, "oauth2client/tracepipeline-test", OAuth2ExtensionID(ref))
}

func TestMakeExtensionConfig(t *testing.T) {
	oauth2Options := &telemetryv1beta1.OAuth2Options{
		TokenURL:     telemetryv1beta1.ValueType{Value: "token-url"},
		ClientID:     telemetryv1beta1.ValueType{Value: "client-id"},
		ClientSecret: telemetryv1beta1.ValueType{Value: "client-secret"},
	}

	ref := TracePipelineRef(&telemetryv1beta1.TracePipeline{ObjectMeta: metav1.ObjectMeta{Name: "test"}})
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
