package endpoint

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/validators/secretref"
)

const (
	FluentdProtocolHTTP = "fluentd-http"
	OTLPProtocolGRPC    = telemetryv1beta1.OTLPProtocolGRPC
	OTLPProtocolHTTP    = telemetryv1beta1.OTLPProtocolHTTP
)

type EndpointValidationParams struct {
	Endpoint   *telemetryv1beta1.ValueType
	Protocol   telemetryv1beta1.OTLPProtocol
	OutputTLS  *telemetryv1beta1.OutputTLS
	OTLPOAuth2 *telemetryv1beta1.OAuth2Options
}

type Validator struct {
	Client client.Reader
}

var (
	ErrValueResolveFailed = errors.New("failed to resolve value")
	ErrPortMissing        = errors.New("missing port")
	ErrUnsupportedScheme  = errors.New("missing or unsupported protocol scheme")
	ErrGRPCOAuth2NoTLS    = errors.New("OAuth2 requires TLS when using gRPC protocol")
	ErrHTTPWithTLS        = errors.New("HTTP scheme with TLS not allowed")
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

func (v *Validator) Validate(ctx context.Context, params EndpointValidationParams) error {
	if params.Endpoint == nil {
		return &EndpointInvalidError{Err: ErrValueResolveFailed}
	}

	endpointValue, err := resolveValue(ctx, v.Client, *params.Endpoint)
	if err != nil {
		return err
	}

	var u *url.URL

	if u, err = parseEndpoint(endpointValue); err != nil {
		return err
	}

	// early return if protocol is Fluentd => further validation is OTLP-exclusive
	if params.Protocol == FluentdProtocolHTTP {
		return nil
	}

	// port validation
	if err := validatePort(u.Host, params.Protocol == OTLPProtocolHTTP); err != nil {
		return err
	}

	// scheme validation
	if params.Protocol == OTLPProtocolHTTP {
		if err := validateSchemeHTTP(u.Scheme); err != nil {
			return err
		}
	}

	// OAuth2 validation
	if params.OTLPOAuth2 != nil {
		var validationFunc func(string, *telemetryv1beta1.OutputTLS) error

		switch params.Protocol {
		case OTLPProtocolGRPC:
			validationFunc = validateGRPCWithOAuth2
		case OTLPProtocolHTTP:
			validationFunc = validateHTTPWithOAuth2
		}

		if err := validationFunc(u.Scheme, params.OutputTLS); err != nil {
			return err
		}
	}

	return nil
}

func resolveValue(ctx context.Context, c client.Reader, value telemetryv1beta1.ValueType) (string, error) {
	if value.Value != "" {
		return value.Value, nil
	}

	if value.ValueFrom == nil || value.ValueFrom.SecretKeyRef == nil {
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

func validateGRPCWithOAuth2(scheme string, tls *telemetryv1beta1.OutputTLS) error {
	// Insecure TLS config
	if tls != nil && tls.Insecure {
		return &EndpointInvalidError{Err: ErrGRPCOAuth2NoTLS}
	}

	// HTTP scheme: invalid in all cases
	if scheme == "http" {
		return &EndpointInvalidError{Err: errors.New(ErrGRPCOAuth2NoTLS.Error() + ": HTTP scheme not allowed")}
	}

	return nil
}

func validateHTTPWithOAuth2(scheme string, tls *telemetryv1beta1.OutputTLS) error {
	// HTTP scheme with TLS
	if scheme == "http" && isTLSConfigured(tls) {
		return &EndpointInvalidError{Err: ErrHTTPWithTLS}
	}

	return nil
}

func isTLSConfigured(tls *telemetryv1beta1.OutputTLS) bool {
	if tls == nil || tls.Insecure {
		return false
	}

	return tls.CA != nil || tls.Cert != nil || tls.Key != nil
}
