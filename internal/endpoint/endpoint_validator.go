package endpoint

import (
	"context"
	"errors"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/secretref"
)

type Validator struct {
	Client client.Reader
}

const (
	ValueResolveFailedErrorMessage = "either value or secret key reference must be provided"
)

type EndpointInvalidError struct {
	Message string
}

func (eie *EndpointInvalidError) Error() string {
	return eie.Message
}

func IsEndpointInvalidError(err error) bool {
	var errEndpointInvalid *EndpointInvalidError
	return errors.As(err, &errEndpointInvalid)
}

func (v *Validator) Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType) error {
	if endpoint == nil {
		return &EndpointInvalidError{Message: ValueResolveFailedErrorMessage}
	}

	endpointValue, err := resolveValue(ctx, v.Client, *endpoint)
	if err != nil {
		return err
	}

	if _, err = parseEndpoint(endpointValue); err != nil {
		return err
	}

	return nil
}

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) ([]byte, error) {
	if value.Value != "" {
		return []byte(value.Value), nil
	}

	if value.ValueFrom == nil || !value.ValueFrom.IsSecretKeyRef() {
		return nil, &EndpointInvalidError{Message: ValueResolveFailedErrorMessage}
	}

	valueFromSecret, err := secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	if err != nil {
		return nil, &EndpointInvalidError{Message: ValueResolveFailedErrorMessage}
	}

	return valueFromSecret, nil
}

func parseEndpoint(endpoint []byte) (*url.URL, error) {
	u, err := url.Parse(string(endpoint))
	if err != nil {
		return nil, &EndpointInvalidError{Message: err.Error()}
	}

	return u, nil
}
