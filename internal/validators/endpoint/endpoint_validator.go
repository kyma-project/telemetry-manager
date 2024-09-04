package endpoint

import (
	"context"
	"errors"
	"net/url"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

type Validator struct {
	Client client.Reader
}

var (
	ErrValueResolveFailed = errors.New("failed to resolve value")
	ErrInvalidPort        = errors.New("missing or invalid port")
)

type EndpointInvalidError struct {
	Err error
}

func (eie *EndpointInvalidError) Error() string {
	return eie.Err.Error()
}

func (eie *EndpointInvalidError) Unwrap() error {
	return eie.Err
}

func IsEndpointInvalidError(err error) bool {
	var errEndpointInvalid *EndpointInvalidError
	return errors.As(err, &errEndpointInvalid)
}

func (v *Validator) Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, withPort bool) error {
	if endpoint == nil {
		return &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	endpointValue, err := resolveValue(ctx, v.Client, *endpoint)
	if err != nil {
		return err
	}

	if _, err = parseEndpoint(endpointValue, withPort); err != nil {
		return err
	}

	return nil
}

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) (string, error) {
	if value.Value != "" {
		return value.Value, nil
	}

	if value.ValueFrom == nil || !value.ValueFrom.IsSecretKeyRef() {
		return "", &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	valueFromSecret, err := secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	if err != nil {
		return "", &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	return string(valueFromSecret), nil
}

func parseEndpoint(endpoint string, withPort bool) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, &EndpointInvalidError{Err: err}
	}

	if withPort {
		port := u.Port()
		if _, err := strconv.Atoi(port); port == "" || err != nil {
			return nil, &EndpointInvalidError{Err: ErrInvalidPort}
		}
	}

	return u, nil
}
