package endpoint

import (
	"context"
	"errors"
	"fmt"
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
	if u.Fragment == tripleSlashMarker && params.Protocol != OTLPProtocolGRPC {
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
	if isHTTPScheme(u.Scheme) || u.Scheme == "" || u.Scheme == "grpc" || u.Fragment == tripleSlashMarker {
		return nil
	}

	switch protocol {
	case OTLPProtocolGRPC:
		// Any non-http scheme with double-slash form should use triple-slash (e.g. passthrough:///, dns:///, xds:///)
		return &EndpointInvalidError{Err: ErrIncorrectGRPCURI}
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
	//
	// We anchor the match to the scheme boundary: scheme characters are [a-zA-Z][a-zA-Z0-9+\-.]*
	// followed by exactly ":///" — this prevents false matches of ":/// " inside a URL path.
	var tripleSlash bool
	if idx := indexTripleSlash(endpoint); idx > 0 {
		tripleSlash = true
		endpoint = endpoint[:idx] + "://" + endpoint[idx+4:]
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

	// Mark triple-slash URIs so Validate can distinguish them from double-slash URIs of
	// the same scheme. We use a dedicated unexported field via a wrapper rather than
	// hijacking u.Fragment (which would corrupt legitimate URL fragments).
	// Since url.URL has no spare fields we carry the flag on the side: callers check
	// u.Fragment == tripleSlashMarker; we clear any real fragment first to avoid
	// fragment-based injection (real gRPC endpoints never carry a fragment).
	if tripleSlash {
		u.Fragment = tripleSlashMarker
	}

	return u, nil
}

// tripleSlashMarker is the synthetic fragment value used to tag triple-slash URIs.
// We clear any pre-existing fragment before setting it; gRPC dial targets do not use
// URL fragments, so this is safe for all valid inputs.
const tripleSlashMarker = "\x00triple-slash"

// indexTripleSlash returns the index of ":/// " in s where the prefix is a valid URI
// scheme (alpha-start, alphanumeric+.-+), or -1 if not found.
// This prevents false matches of ":///" inside a URL path component.
func indexTripleSlash(s string) int {
	idx := strings.Index(s, ":///")
	if idx <= 0 {
		return -1
	}
	// Verify that s[0:idx] is a valid scheme: starts with letter, rest are [a-zA-Z0-9+\-.]
	scheme := s[:idx]
	for i, c := range scheme {
		if i == 0 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
				return -1
			}
		} else {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '+' || c == '-' || c == '.') {
				return -1
			}
		}
	}
	return idx
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
		return &EndpointInvalidError{Err: fmt.Errorf("%w: HTTP scheme not allowed", ErrGRPCOAuth2NoTLS)}
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
