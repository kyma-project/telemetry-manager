package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	sharedtypesutils "github.com/kyma-project/telemetry-manager/internal/utils/sharedtypes"
)

const (
	basicAuthHeaderVariablePrefix    = "BASIC_AUTH_HEADER"
	otlpEndpointVariablePrefix       = "OTLP_ENDPOINT"
	tlsConfigCertVariablePrefix      = "OTLP_TLS_CERT_PEM"
	tlsConfigKeyVariablePrefix       = "OTLP_TLS_KEY_PEM"
	tlsConfigCaVariablePrefix        = "OTLP_TLS_CA_PEM"
	oauth2TokenURLVariablePrefix     = "OAUTH2_TOKEN_URL"     //nolint:gosec // G101: This is a variable name prefix, not a credential
	oauth2ClientIDVariablePrefix     = "OAUTH2_CLIENT_ID"     //nolint:gosec // G101: This is a variable name prefix, not a credential
	oauth2ClientSecretVariablePrefix = "OAUTH2_CLIENT_SECRET" //nolint:gosec // G101: This is a variable name prefix, not a credential
)

// =============================================================================
// Env Vars Builders
// =============================================================================

func makeOTLPExporterEnvVars(ctx context.Context, c client.Reader, output *telemetryv1beta1.OTLPOutput, pipelineName string, signalType SignalType) (map[string][]byte, error) {
	var err error

	secretData := make(map[string][]byte)

	err = makeBasicAuthEnvVar(ctx, c, secretData, output, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	err = makeOTLPEndpointEnvVar(ctx, c, secretData, output, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	err = makeHeaderEnvVar(ctx, c, secretData, output, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	err = makeTLSEnvVar(ctx, c, secretData, output, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	return secretData, nil
}

func makeOAuth2ExtensionEnvVars(ctx context.Context, c client.Reader, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineName string, signalType SignalType) (map[string][]byte, error) {
	var err error

	secretData := make(map[string][]byte)

	err = makeTokenURLEnvVar(ctx, c, secretData, oauth2Options, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	err = makeClientIDEnvVar(ctx, c, secretData, oauth2Options, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	err = makeClientSecretEnvVar(ctx, c, secretData, oauth2Options, pipelineName, signalType)
	if err != nil {
		return nil, err
	}

	return secretData, nil
}

func makeBasicAuthEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineName string, signalType SignalType) error {
	if isBasicAuthEnabled(output.Authentication) {
		username, err := sharedtypesutils.ResolveValue(ctx, c, output.Authentication.Basic.User)
		if err != nil {
			return err
		}

		password, err := sharedtypesutils.ResolveValue(ctx, c, output.Authentication.Basic.Password)
		if err != nil {
			return err
		}

		basicAuthHeader := formatBasicAuthHeader(string(username), string(password))
		basicAuthHeaderVariable := formatEnvVarKey(basicAuthHeaderVariablePrefix, signalType, pipelineName)
		secretData[basicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	return nil
}

func makeOTLPEndpointEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineName string, signalType SignalType) error {
	otlpEndpointVariable := formatEnvVarKey(otlpEndpointVariablePrefix, signalType, pipelineName)

	endpointURL, err := resolveEndpointURL(ctx, c, output)
	if err != nil {
		return err
	}

	secretData[otlpEndpointVariable] = endpointURL

	return err
}

func makeHeaderEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineName string, signalType SignalType) error {
	for _, header := range output.Headers {
		key := formatHeaderEnvVarKey(header, signalType, pipelineName)

		value, err := sharedtypesutils.ResolveValue(ctx, c, header.ValueType)
		if err != nil {
			return err
		}

		secretData[key] = prefixHeaderValue(header, value)
	}

	return nil
}

func makeTLSEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineName string, signalType SignalType) error {
	if output.TLS != nil {
		if sharedtypesutils.IsValid(output.TLS.CA) {
			ca, err := sharedtypesutils.ResolveValue(ctx, c, *output.TLS.CA)
			if err != nil {
				return err
			}

			tlsConfigCaVariable := formatEnvVarKey(tlsConfigCaVariablePrefix, signalType, pipelineName)
			secretData[tlsConfigCaVariable] = ca
		}

		if sharedtypesutils.IsValid(output.TLS.Cert) && sharedtypesutils.IsValid(output.TLS.Key) {
			cert, err := sharedtypesutils.ResolveValue(ctx, c, *output.TLS.Cert)
			if err != nil {
				return err
			}

			key, err := sharedtypesutils.ResolveValue(ctx, c, *output.TLS.Key)
			if err != nil {
				return err
			}

			// Make a best effort replacement of linebreaks in cert/key if present.
			sanitizedCert := bytes.ReplaceAll(cert, []byte("\\n"), []byte("\n"))
			sanitizedKey := bytes.ReplaceAll(key, []byte("\\n"), []byte("\n"))

			tlsConfigCertVariable := formatEnvVarKey(tlsConfigCertVariablePrefix, signalType, pipelineName)
			secretData[tlsConfigCertVariable] = sanitizedCert

			tlsConfigKeyVariable := formatEnvVarKey(tlsConfigKeyVariablePrefix, signalType, pipelineName)
			secretData[tlsConfigKeyVariable] = sanitizedKey
		}
	}

	return nil
}

func makeTokenURLEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineName string, signalType SignalType) error {
	if oauth2Options != nil && sharedtypesutils.IsValid(&oauth2Options.TokenURL) {
		tokenURL, err := sharedtypesutils.ResolveValue(ctx, c, oauth2Options.TokenURL)
		if err != nil {
			return err
		}

		tokenURLVariable := formatEnvVarKey(oauth2TokenURLVariablePrefix, signalType, pipelineName)
		secretData[tokenURLVariable] = tokenURL
	}

	return nil
}

func makeClientIDEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineName string, signalType SignalType) error {
	if oauth2Options != nil && sharedtypesutils.IsValid(&oauth2Options.ClientID) {
		clientID, err := sharedtypesutils.ResolveValue(ctx, c, oauth2Options.ClientID)
		if err != nil {
			return err
		}

		clientIDVariable := formatEnvVarKey(oauth2ClientIDVariablePrefix, signalType, pipelineName)
		secretData[clientIDVariable] = clientID
	}

	return nil
}

func makeClientSecretEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineName string, signalType SignalType) error {
	if oauth2Options != nil && sharedtypesutils.IsValid(&oauth2Options.ClientSecret) {
		clientSecret, err := sharedtypesutils.ResolveValue(ctx, c, oauth2Options.ClientSecret)
		if err != nil {
			return err
		}

		clientSecretVariable := formatEnvVarKey(oauth2ClientSecretVariablePrefix, signalType, pipelineName)
		secretData[clientSecretVariable] = clientSecret
	}

	return nil
}

// =============================================================================
// Helper Functions
// =============================================================================

func prefixHeaderValue(header telemetryv1beta1.Header, value []byte) []byte {
	if len(strings.TrimSpace(header.Prefix)) > 0 {
		return fmt.Appendf(nil, "%s %s", strings.TrimSpace(header.Prefix), string(value))
	}

	return value
}

func resolveEndpointURL(ctx context.Context, c client.Reader, output *telemetryv1beta1.OTLPOutput) ([]byte, error) {
	endpoint, err := sharedtypesutils.ResolveValue(ctx, c, output.Endpoint)
	if err != nil {
		return nil, err
	}

	if len(output.Path) > 0 {
		u, err := url.Parse(string(endpoint))
		if err != nil {
			return nil, err
		}

		pathRef, err := url.Parse(output.Path)
		if err != nil {
			return nil, err
		}

		u.Path = path.Join(u.Path, pathRef.Path)
		u.RawQuery = pathRef.RawQuery

		return []byte(u.String()), nil
	}

	return endpoint, nil
}

func formatBasicAuthHeader(username string, password string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}

func formatEnvVarKey(prefix string, signalType SignalType, pipelineName string) string {
	return fmt.Sprintf("%s_%s_%s", prefix, sanitizeEnvVarName(string(signalType)), sanitizeEnvVarName(pipelineName))
}

func formatHeaderEnvVarKey(header telemetryv1beta1.Header, signalType SignalType, pipelineName string) string {
	return fmt.Sprintf("HEADER_%s_%s_%s", sanitizeEnvVarName(string(signalType)), sanitizeEnvVarName(pipelineName), sanitizeEnvVarName(header.Name))
}

func sanitizeEnvVarName(input string) string {
	result := input
	result = strings.ToUpper(result)
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "-", "_")

	return result
}
