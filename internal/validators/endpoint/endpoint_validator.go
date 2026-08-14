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
	ErrIncorrectGRPCURI   = errors.New("incorrect gRPC URI")
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

	// Detect the original scheme before parseEndpoint normalizes it.
	// parseEndpoint clears the scheme for triple-slash URIs (scheme:///host:port) after
	// re-parsing so that port validation works, but we still need to know the original scheme
	// to reject non-http/https URIs for protocols that don't support them.
	originalScheme := originalURIScheme(endpointValue)

	// Reject unknown non-http/https schemes early, before port validation.
	// "grpc" is exempted: port error takes precedence for a better user message.
	// Triple-slash URIs have their scheme cleared by parseEndpoint (u.Scheme==""), so they
	// are handled separately below after port validation.
	if !isHTTPScheme(u.Scheme) && u.Scheme != "" && u.Scheme != "grpc" {
		switch params.Protocol {
		case OTLPProtocolGRPC:
			if u.Scheme == "passthrough" {
				return &EndpointInvalidError{Err: ErrIncorrectGRPCURI}
			}

			return &EndpointInvalidError{Err: ErrUnsupportedScheme}
		default:
			return &EndpointInvalidError{Err: ErrUnsupportedScheme}
		}
	}

	// port validation for all protocols
	// Fluentd HTTP allows missing port (will use default)
	var allowMissingPort = true

	// OTLP gRPC requires port to be specified
	if params.Protocol == OTLPProtocolGRPC {
		allowMissingPort = false
	}

	if err := validatePort(u.Host, allowMissingPort); err != nil {
		return err
	}

	// For triple-slash URIs (scheme cleared by parseEndpoint), reject non-http/https schemes
	// for protocols that don't support gRPC resolver URIs.
	if !isHTTPScheme(originalScheme) && originalScheme != "" && u.Scheme == "" {
		if params.Protocol == OTLPProtocolHTTP || params.Protocol == FluentdProtocolHTTP {
			return &EndpointInvalidError{Err: ErrUnsupportedScheme}
		}
	}

	// "grpc" scheme: port was checked above (error takes precedence), now reject the scheme itself.
	if u.Scheme == "grpc" {
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
	}

	// early return if protocol is Fluentd => further validation is OTLP-exclusive
	if params.Protocol == FluentdProtocolHTTP {
		return nil
	}

	// scheme validation
	if params.Protocol == OTLPProtocolHTTP {
		if err := validateSchemeHTTP(u.Scheme); err != nil {
			return err
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

	// scheme present but no authority at all — invalid regardless of path.
	// Only applies when the input looks like a genuine URI (contains "://") but has nothing
	// after the authority separator, i.e. "scheme://" or "scheme:///".
	// Schemeless inputs like "example.com:" are handled below via the placeholder path.
	if strings.Contains(endpoint, "://") && u.Scheme != "" && u.Opaque == "" && u.Host == "" && strings.TrimPrefix(u.Path, "/") == "" {
		return nil, &EndpointInvalidError{Err: ErrPortMissing}
	}

	// gRPC URI form: scheme:///authority (triple slash, empty host, endpoint in path).
	// Re-parse treating the path as the host so port validation works correctly.
	// The scheme is cleared so downstream checks that restrict non-http schemes don't fire —
	// the scheme is preserved verbatim in the env var; we only need host:port here.
	if u.Scheme != "" && u.Host == "" && u.Path != "" {
		rewritten := u.Scheme + "://" + strings.TrimPrefix(u.Path, "/")
		u, err = url.Parse(rewritten)
		if err != nil {
			return nil, &EndpointInvalidError{Err: err}
		}

		u.Scheme = ""
		return u, nil
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

// isHTTPScheme returns true for the only schemes that may carry the endpoint in the authority (host field).
func isHTTPScheme(scheme string) bool {
	return scheme == "http" || scheme == "https"
}

// originalURIScheme extracts the scheme from a URI string without full parsing.
// Returns "" for schemeless inputs (no "://").
func originalURIScheme(endpoint string) string {
	if idx := strings.Index(endpoint, "://"); idx > 0 {
		return endpoint[:idx]
	}

	return ""
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

func validateSchemeHTTP(scheme string) error {
	if scheme != "http" && scheme != "https" {
		return &EndpointInvalidError{Err: ErrUnsupportedScheme}
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
