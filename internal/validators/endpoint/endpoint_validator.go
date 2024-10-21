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

const (
	FluentdProtocolHTTP = "fluentd-http"
	OtlpProtocolGRPC    = telemetryv1alpha1.OtlpProtocolGRPC
	OtlpProtocolHTTP    = telemetryv1alpha1.OtlpProtocolHTTP
)

type Validator struct {
	Client client.Reader
}

var (
	ErrValueResolveFailed = errors.New("failed to resolve value")
	ErrPortMissing        = errors.New("missing port")
	ErrUnsupportedScheme  = errors.New("missing or unsupported protocol scheme")
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

func (v *Validator) Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, protocol string) error {
	if endpoint == nil {
		return &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	endpointValue, err := resolveValue(ctx, v.Client, *endpoint)
	if err != nil {
		return err
	}

	var u *url.URL

	if u, err = parseEndpoint(endpointValue); err != nil {
		return err
	}

	if protocol == FluentdProtocolHTTP {
		return nil
	}

	var hostport = u.Host + u.Path
	if err := validatePort(hostport, protocol == OtlpProtocolHTTP); err != nil {
		return err
	}

	if protocol == OtlpProtocolHTTP {
		if err := validateSchemeHTTP(u.Scheme); err != nil {
			return err
		}
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

func parseEndpoint(endpoint string) (*url.URL, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, &EndpointInvalidError{Err: err}
	}

	// parse a URL without scheme
	if u.Opaque != "" || u.Scheme == "" || u.Host == "" {
		const placeholder = "plhd://"

		u, err = url.Parse(placeholder + endpoint)
		if err != nil {
			errMsg := strings.Replace(err.Error(), placeholder, "", 1)
			return nil, &EndpointInvalidError{Err: errors.New(errMsg)}
		}

		u.Scheme = ""
	}

	return u, nil
}

func validatePort(hostport string, allowMissing bool) error {
	_, port, err := net.SplitHostPort(hostport)
	if err != nil && strings.Contains(err.Error(), "missing port in address") {
		if !allowMissing {
			return &EndpointInvalidError{Err: ErrPortMissing}
		} else {
			return nil
		}
	} else if err != nil {
		return &EndpointInvalidError{Err: err}
	}

	if allowMissing {
		return nil
	}

	if _, err := strconv.Atoi(port); port == "" || err != nil {
		return &EndpointInvalidError{Err: ErrPortMissing}
	}

	return nil
}

func validateSchemeHTTP(scheme string) error {
	if scheme != "http" && scheme != "https" {
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	}

	return nil
}
