package builder

import (
	"context"
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"
)

const (
	basicAuthHeaderVariable = "BASIC_AUTH_HEADER"
	otlpEndpointVariable    = "OTLP_ENDPOINT"
)

func makeHeaderEnvVarCompliant(header telemetryv1alpha1.Header) string {
	return fmt.Sprintf("${HEADER_%s}", envvar.MakeEnvVarCompliant(header.Name))
}

func makeEnvVars(ctx context.Context, c client.Reader, output *telemetryv1alpha1.OtlpOutput) (map[string][]byte, error) {
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
		secretData[basicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	endpoint, err := resolveValue(ctx, c, output.Endpoint)
	if err != nil {
		return nil, err
	}
	secretData[otlpEndpointVariable] = endpoint

	for _, header := range output.Headers {
		key := makeHeaderEnvVarCompliant(header)
		value, err := resolveValue(ctx, c, header.ValueType)
		if err != nil {
			return nil, err
		}
		secretData[key] = value
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
