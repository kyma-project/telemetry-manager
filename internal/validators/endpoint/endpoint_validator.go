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
	ErrGRPCWithPath       = errors.New("gRPC endpoints cannot contain paths")
	ErrIncorrectGRPCURI   = errors.New("incorrect gRPC URI: use triple-slash form (e.g. passthrough:///host:port)")
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

	if err := validateScheme(u, params.Protocol); err != nil {
		return err
	}

	allowMissingPort := params.Protocol != OTLPProtocolGRPC
	if err := validatePort(u.Host, allowMissingPort); err != nil {
		return err
	}

	// "grpc" scheme: port error takes precedence (checked above), now reject the scheme.
	if u.Scheme == "grpc" {
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	}

	// For triple-slash gRPC URIs (scheme:///authority) the collector passes them verbatim to
	// grpc.NewClient, which supports any registered resolver scheme (passthrough, dns, xds, etc.).
	// All triple-slash schemes are therefore valid for gRPC; for HTTP and Fluentd they are not.
	if u.Fragment == "triple-slash" && params.Protocol != OTLPProtocolGRPC {
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	}

	// early return if protocol is Fluentd => further validation is OTLP-exclusive
	if params.Protocol == FluentdProtocolHTTP {
		return nil
	}

	// scheme validation for OTLP HTTP
	if params.Protocol == OTLPProtocolHTTP {
		if !isHTTPScheme(u.Scheme) {
			return &EndpointInvalidError{Err: ErrUnsupportedScheme}
		}
	}

	// path validation for gRPC
	if params.Protocol == OTLPProtocolGRPC {
		if err := validateGRPCPath(u.Path); err != nil {
			return err
		}
	}

	// OAuth2 validation
	if params.OTLPOAuth2 != nil {
		if err := validateOAuth2(u.Scheme, params.Protocol, params.OutputTLS); err != nil {
			return err
		}
	}

	return nil
}

// validateScheme rejects non-http/https double-slash URIs before port validation.
// "grpc" scheme is exempted so port errors take precedence for a better user message.
// Triple-slash URIs are handled separately after port validation.
func validateScheme(u *url.URL, protocol telemetryv1beta1.OTLPProtocol) error {
	if isHTTPScheme(u.Scheme) || u.Scheme == "" || u.Scheme == "grpc" || u.Fragment == "triple-slash" {
		return nil
	}

	switch protocol {
	case OTLPProtocolGRPC:
		if u.Scheme == "passthrough" {
			return &EndpointInvalidError{Err: ErrIncorrectGRPCURI}
		}

		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	case OTLPProtocolHTTP:
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	default:
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	}
}

func validateOAuth2(scheme string, protocol telemetryv1beta1.OTLPProtocol, tls *telemetryv1beta1.OutputTLS) error {
	switch protocol {
	case OTLPProtocolGRPC:
		return validateGRPCWithOAuth2(scheme, tls)
	case OTLPProtocolHTTP:
		return validateHTTPWithOAuth2(scheme, tls)
	default:
		return nil
	}
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
	// Normalize triple-slash gRPC URI form (scheme:///authority) to scheme://authority
	// so that url.Parse puts the authority in u.Host and preserves u.Scheme.
	// This avoids the need to clear and restore the scheme after parsing, and prevents
	// single-slash URIs (scheme:/path) from being misidentified as triple-slash forms.
	tripleSlash := strings.Contains(endpoint, ":///")
	if tripleSlash {
		if idx := strings.Index(endpoint, ":///"); idx > 0 {
			endpoint = endpoint[:idx] + "://" + endpoint[idx+4:]
		}
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, &EndpointInvalidError{Err: err}
	}

	// Reject scheme://  or scheme:/// (no authority at all) — only applies to real URIs
	// (contains "://"), not to schemeless inputs like "example.com:" handled below.
	if strings.Contains(endpoint, "://") && u.Scheme != "" && u.Opaque == "" && u.Host == "" && strings.TrimPrefix(u.Path, "/") == "" {
		return nil, &EndpointInvalidError{Err: ErrPortMissing}
	}

	// Parse schemeless input or opaque URL by prepending a placeholder scheme.
	if u.Opaque != "" || u.Scheme == "" || u.Host == "" {
		const placeholder = "plhd://"

		u, err = url.Parse(placeholder + endpoint)
		if err != nil {
			errMsg := strings.Replace(err.Error(), placeholder, "", 1)
			return nil, &EndpointInvalidError{Err: errors.New(errMsg)}
		}

		u.Scheme = ""
	}

	// Mark triple-slash URIs with a synthetic fragment so Validate can distinguish them
	// from double-slash URIs of the same scheme.
	if tripleSlash {
		u.Fragment = "triple-slash"
	}

	return u, nil
}

// isHTTPScheme returns true for schemes that carry the endpoint in the authority (host field).
func isHTTPScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
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

	// In case OTLP GRPC it is important to pass the port.
	if _, err := strconv.Atoi(port); port == "" || err != nil {
		return &EndpointInvalidError{Err: ErrPortMissing}
	}

	return nil
}

func validateGRPCPath(path string) error {
	// OTel Collector's gRPC exporter does not accept any path, including trailing slashes.
	if path != "" {
		return &EndpointInvalidError{Err: ErrGRPCWithPath}
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
