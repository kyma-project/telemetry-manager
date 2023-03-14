package secretref

import (
	"context"
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/utils/envvar"
)

const (
	BasicAuthHeaderVariable = "BASIC_AUTH_HEADER"
	OtlpEndpointVariable    = "OTLP_ENDPOINT"
)

func FetchDataForOtlpOutput(ctx context.Context, client client.Reader, output *telemetryv1alpha1.OtlpOutput) (map[string][]byte, error) {
	secretData := map[string][]byte{}

	if output.Authentication != nil && output.Authentication.Basic.IsDefined() {
		username, err := fetchSecretValue(ctx, client, output.Authentication.Basic.User)
		if err != nil {
			return nil, err
		}
		password, err := fetchSecretValue(ctx, client, output.Authentication.Basic.Password)
		if err != nil {
			return nil, err
		}
		basicAuthHeader := getBasicAuthHeader(string(username), string(password))
		secretData[BasicAuthHeaderVariable] = []byte(basicAuthHeader)
	}

	endpoint, err := fetchSecretValue(ctx, client, output.Endpoint)
	if err != nil {
		return nil, err
	}
	secretData[OtlpEndpointVariable] = endpoint

	for _, header := range output.Headers {
		key := fmt.Sprintf("HEADER_%s", envvar.MakeEnvVarCompliant(header.Name))
		value, err := fetchSecretValue(ctx, client, header.ValueType)
		if err != nil {
			return nil, err
		}
		secretData[key] = value
	}

	return secretData, nil
}

func getBasicAuthHeader(username string, password string) string {
	return fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(username+":"+password)))
}
