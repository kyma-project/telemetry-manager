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

func makeOTLPExporterEnvVars(ctx context.Context, c client.Reader, output *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef) (map[string][]byte, error) {
	var err error

	secretData := make(map[string][]byte)

	err = makeBasicAuthEnvVar(ctx, c, secretData, output, pipelineRef)
	if err != nil {
		return nil, err
	}

	err = makeOTLPEndpointEnvVar(ctx, c, secretData, output, pipelineRef)
	if err != nil {
		return nil, err
	}

	err = makeHeaderEnvVar(ctx, c, secretData, output, pipelineRef)
	if err != nil {
		return nil, err
	}

	err = makeTLSEnvVar(ctx, c, secretData, output, pipelineRef)
	if err != nil {
		return nil, err
	}

	return secretData, nil
}

func makeOAuth2ExtensionEnvVars(ctx context.Context, c client.Reader, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineRef PipelineRef) (map[string][]byte, error) {
	var err error

	secretData := make(map[string][]byte)

	err = makeTokenURLEnvVar(ctx, c, secretData, oauth2Options, pipelineRef)
	if err != nil {
		return nil, err
	}

	err = makeClientIDEnvVar(ctx, c, secretData, oauth2Options, pipelineRef)
	if err != nil {
		return nil, err
	}

	err = makeClientSecretEnvVar(ctx, c, secretData, oauth2Options, pipelineRef)
	if err != nil {
		return nil, err
	}

	return secretData, nil
}

func makeBasicAuthEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef) error {
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
		basicAuthHeaderVariable := formatEnvVarKey(basicAuthHeaderVariablePrefix, pipelineRef)
		secretData[basicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	return nil
}

func makeOTLPEndpointEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef) error {
	otlpEndpointVariable := formatEnvVarKey(otlpEndpointVariablePrefix, pipelineRef)

	endpointURL, err := resolveEndpointURL(ctx, c, output)
	if err != nil {
		return err
	}

	secretData[otlpEndpointVariable] = endpointURL

	return err
}

func makeHeaderEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef) error {
	for _, header := range output.Headers {
		key := formatHeaderEnvVarKey(header, pipelineRef)

		value, err := sharedtypesutils.ResolveValue(ctx, c, header.ValueType)
		if err != nil {
			return err
		}

		secretData[key] = prefixHeaderValue(header, value)
	}

	return nil
}

func makeTLSEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, output *telemetryv1beta1.OTLPOutput, pipelineRef PipelineRef) error {
	if output.TLS != nil {
		if sharedtypesutils.IsValid(output.TLS.CA) {
			ca, err := sharedtypesutils.ResolveValue(ctx, c, *output.TLS.CA)
			if err != nil {
				return err
			}

			tlsConfigCaVariable := formatEnvVarKey(tlsConfigCaVariablePrefix, pipelineRef)
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

			tlsConfigCertVariable := formatEnvVarKey(tlsConfigCertVariablePrefix, pipelineRef)
			secretData[tlsConfigCertVariable] = sanitizedCert

			tlsConfigKeyVariable := formatEnvVarKey(tlsConfigKeyVariablePrefix, pipelineRef)
			secretData[tlsConfigKeyVariable] = sanitizedKey
		}
	}

	return nil
}

func makeTokenURLEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineRef PipelineRef) error {
	if oauth2Options != nil && sharedtypesutils.IsValid(&oauth2Options.TokenURL) {
		tokenURL, err := sharedtypesutils.ResolveValue(ctx, c, oauth2Options.TokenURL)
		if err != nil {
			return err
		}

		tokenURLVariable := formatEnvVarKey(oauth2TokenURLVariablePrefix, pipelineRef)
		secretData[tokenURLVariable] = tokenURL
	}

	return nil
}

func makeClientIDEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineRef PipelineRef) error {
	if oauth2Options != nil && sharedtypesutils.IsValid(&oauth2Options.ClientID) {
		clientID, err := sharedtypesutils.ResolveValue(ctx, c, oauth2Options.ClientID)
		if err != nil {
			return err
		}

		clientIDVariable := formatEnvVarKey(oauth2ClientIDVariablePrefix, pipelineRef)
		secretData[clientIDVariable] = clientID
	}

	return nil
}

func makeClientSecretEnvVar(ctx context.Context, c client.Reader, secretData map[string][]byte, oauth2Options *telemetryv1beta1.OAuth2Options, pipelineRef PipelineRef) error {
	if oauth2Options != nil && sharedtypesutils.IsValid(&oauth2Options.ClientSecret) {
		clientSecret, err := sharedtypesutils.ResolveValue(ctx, c, oauth2Options.ClientSecret)
		if err != nil {
			return err
		}

		clientSecretVariable := formatEnvVarKey(oauth2ClientSecretVariablePrefix, pipelineRef)
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

// formatEnvVarKey builds an environment variable key for a pipeline.
// Example: Type="trace" → "PREFIX_TRACEPIPELINE_PIPELINENAME"
// Example: Type=""      → "PREFIX_PIPELINENAME"
func formatEnvVarKey(prefix string, pipelineRef PipelineRef) string {
	if tp := pipelineRef.typePrefix(); tp != "" {
		return fmt.Sprintf("%s_%s_%s", prefix, sanitizeEnvVarName(tp), sanitizeEnvVarName(pipelineRef.Name))
	}

	return fmt.Sprintf("%s_%s", prefix, sanitizeEnvVarName(pipelineRef.Name))
}

// formatHeaderEnvVarKey builds an environment variable key for a custom header.
// Example: Type="trace" → "HEADER_TRACEPIPELINE_PIPELINENAME_HEADERNAME"
// Example: Type=""      → "HEADER_PIPELINENAME_HEADERNAME"
func formatHeaderEnvVarKey(header telemetryv1beta1.Header, pipelineRef PipelineRef) string {
	if tp := pipelineRef.typePrefix(); tp != "" {
		return fmt.Sprintf("HEADER_%s_%s_%s", sanitizeEnvVarName(tp), sanitizeEnvVarName(pipelineRef.Name), sanitizeEnvVarName(header.Name))
	}

	return fmt.Sprintf("HEADER_%s_%s", sanitizeEnvVarName(pipelineRef.Name), sanitizeEnvVarName(header.Name))
}

func sanitizeEnvVarName(input string) string {
	result := input
	result = strings.ToUpper(result)
	result = strings.ReplaceAll(result, ".", "_")
	result = strings.ReplaceAll(result, "-", "_")

	return result
}
