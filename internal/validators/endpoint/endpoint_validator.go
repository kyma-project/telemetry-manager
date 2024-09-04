package endpoint

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

type Endpoint struct {
	Endpoint *telemetryv1alpha1.ValueType
	Host     *telemetryv1alpha1.ValueType
	Port     string
}

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

func (v *Validator) Validate(ctx context.Context, endpoint Endpoint) error {
	var host, port string
	var err error
	if endpoint.Endpoint != nil {
		if host, port, err = resolveValueEndpoint(ctx, v.Client, *endpoint.Endpoint); err != nil {
			return &EndpointInvalidError{Err: err}
		}
	} else if endpoint.Host != nil {
		if host, err = resolveValueHost(ctx, v.Client, *endpoint.Host); err != nil {
			return &EndpointInvalidError{Err: err}
		}
		port = endpoint.Port
	} else {
		return &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	if _, err = parseEndpoint(host, port); err != nil {
		return err
	}

	return nil
}

func resolveValueEndpoint(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) (string, string, error) {
	if value.Value != "" {
		if host, port, err := net.SplitHostPort(value.Value); err == nil {
			return "", "", &EndpointInvalidError{Err: ErrValueResolveFailed}
		} else {
			return host, port, nil
		}
	}

	if value.ValueFrom == nil || !value.ValueFrom.IsSecretKeyRef() {
		return "", "", &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	valueFromSecret, err := secretref.GetValue(ctx, c, *value.ValueFrom.SecretKeyRef)
	if err != nil {
		return "", "", &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	if host, port, err := net.SplitHostPort(string(valueFromSecret)); err == nil {
		return "", "", &EndpointInvalidError{Err: ErrValueResolveFailed}
	} else {
		return host, port, nil
	}
}

func resolveValueHost(ctx context.Context, c client.Reader, value telemetryv1alpha1.ValueType) (string, error) {
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

func parseEndpoint(host string, port string) (*url.URL, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, &EndpointInvalidError{Err: err}
	}

	if _, err := strconv.Atoi(port); port == "" || err != nil {
		return nil, &EndpointInvalidError{Err: ErrInvalidPort}
	}

	return u, nil
}
