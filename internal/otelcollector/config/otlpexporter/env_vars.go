package otlpexporter

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"path"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"
)

const (
	basicAuthHeaderVariablePrefix = "BASIC_AUTH_HEADER"
	otlpEndpointVariablePrefix    = "OTLP_ENDPOINT"
	tlsConfigCertVariablePrefix   = "OTLP_TLS_CERT_PEM"
	tlsConfigKeyVariablePrefix    = "OTLP_TLS_KEY_PEM"
	tlsConfigCaVariablePrefix     = "OTLP_TLS_CA_PEM"
)

func makeEnvVars(ctx context.Context, c client.Reader, output *telemetryv1alpha1.OtlpOutput, pipelineName string) (map[string][]byte, error) {
	secretData := make(map[string][]byte)
	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		username, err := resolveValue(ctx, c, output.Authentication.Basic.User)
		if err != nil {
			return nil, err
		}
		password, err := resolveValue(ctx, c, output.Authentication.Basic.Password)
		if err != nil {
			return nil, err
		}
		basicAuthHeader := formatBasicAuthHeader(string(username), string(password))
		basicAuthHeaderVariable := fmt.Sprintf("%s_%s", basicAuthHeaderVariablePrefix, envvar.MakeEnvVarCompliant(pipelineName))
		secretData[basicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	endpoint, err := resolveValue(ctx, c, output.Endpoint)
	if err != nil {
		return nil, err
	}
	otlpEndpointVariable := makeOtlpEndpointVariable(pipelineName)
	secretData[otlpEndpointVariable] = endpoint

	if len(output.Path) > 0 {
		u, err := url.Parse(string(endpoint))
		if err != nil {
			return nil, err
		}
		u.Path = path.Join(u.Path, output.Path)
		secretData[otlpEndpointVariable] = []byte(u.String())
	}

	for _, header := range output.Headers {
		key := makeHeaderVariable(header, pipelineName)
		value, err := resolveValue(ctx, c, header.ValueType)
		if err != nil {
			return nil, err
		}
		secretData[key] = value
	}

	if output.TLS != nil {
		if output.TLS.CA.IsDefined() {
			ca, err := resolveValue(ctx, c, *output.TLS.CA)
			if err != nil {
				return nil, err
			}
			tlsConfigCaVariable := makeTLSCaVariable(pipelineName)
			secretData[tlsConfigCaVariable] = ca
		}
		if output.TLS.Cert.IsDefined() {
			cert, err := resolveValue(ctx, c, *output.TLS.Cert)
			if err != nil {
				return nil, err
			}
			tlsConfigCertVariable := makeTLSCertVariable(pipelineName)
			secretData[tlsConfigCertVariable] = cert
		}
		if output.TLS.Key.IsDefined() {
			key, err := resolveValue(ctx, c, *output.TLS.Key)
			if err != nil {
				return nil, err
			}
			tlsConfigKeyVariable := makeTLSKeyVariable(pipelineName)
			secretData[tlsConfigKeyVariable] = key
		}
	}

	return secretData, nil
}

func formatBasicAuthHeader(username string, password string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}
	if value.ValueFrom.IsSecretKeyRef() {
		return secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	}

	return nil, fmt.Errorf("either value or secret key reference must be defined")
}

func makeOtlpEndpointVariable(pipelineName string) string {
	return fmt.Sprintf("%s_%s", otlpEndpointVariablePrefix, envvar.MakeEnvVarCompliant(pipelineName))
}

func makeBasicAuthHeaderVariable(pipelineName string) string {
	return fmt.Sprintf("%s_%s", basicAuthHeaderVariablePrefix, envvar.MakeEnvVarCompliant(pipelineName))
}

func makeHeaderVariable(header telemetryv1alpha1.Header, pipelineName string) string {
	return fmt.Sprintf("HEADER_%s_%s", envvar.MakeEnvVarCompliant(pipelineName), envvar.MakeEnvVarCompliant(header.Name))
}

func makeTLSCertVariable(pipelineName string) string {
	return fmt.Sprintf("%s_%s", tlsConfigCertVariablePrefix, envvar.MakeEnvVarCompliant(pipelineName))
}

func makeTLSKeyVariable(pipelineName string) string {
	return fmt.Sprintf("%s_%s", tlsConfigKeyVariablePrefix, envvar.MakeEnvVarCompliant(pipelineName))
}

func makeTLSCaVariable(pipelineName string) string {
	return fmt.Sprintf("%s_%s", tlsConfigCaVariablePrefix, envvar.MakeEnvVarCompliant(pipelineName))
}
