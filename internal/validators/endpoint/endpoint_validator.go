package endpoint

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

const schemePlaceholder = "grpc://"

type Validator struct {
	Client client.Reader
}

var (
	ErrValueResolveFailed = errors.New("failed to resolve value")
	ErrPortMissing        = errors.New("missing port")
	ErrPortInvalid        = errors.New("invalid port")
)

type EndpointInvalidError struct {
	Err                     error
	RemoveSchemePlaceholder bool
}

func (eie *EndpointInvalidError) Error() string {
	errMessage := eie.Err.Error()

	if eie.RemoveSchemePlaceholder {
		return strings.Replace(errMessage, schemePlaceholder, "", 1)
	}

	return errMessage
}

func (eie *EndpointInvalidError) Unwrap() error {
	return eie.Err
}

func IsEndpointInvalidError(err error) bool {
	var errEndpointInvalid *EndpointInvalidError
	return errors.As(err, &errEndpointInvalid)
}

func (v *Validator) Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, validatePort bool) error {
	if endpoint == nil {
		return &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	endpointValue, err := resolveValue(ctx, v.Client, *endpoint)
	if err != nil {
		return err
	}

	if _, err = parseEndpoint(endpointValue, validatePort); err != nil {
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

func parseEndpoint(endpoint string, validatePort bool) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, &EndpointInvalidError{Err: err}
	} else if u.Opaque != "" {
		u, err = url.Parse(schemePlaceholder + endpoint) // parse a URL without scheme
		if err != nil {
			return nil, &EndpointInvalidError{Err: err, RemoveSchemePlaceholder: true}
		}
	}

	if !validatePort {
		return u, nil
	}

	_, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, &EndpointInvalidError{Err: err}
	}

	if port == "" {
		return nil, &EndpointInvalidError{Err: ErrPortMissing}
	}

	if _, err := strconv.Atoi(port); err != nil {
		return nil, &EndpointInvalidError{Err: ErrPortInvalid}
	}

	return u, nil
}
