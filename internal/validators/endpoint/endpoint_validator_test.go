package endpoint

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

const (
	errMsgEndpointInvalid         = "parse \"%s\": first path segment in URL cannot contain colon"
	errMsgEndpointResolveFailed   = "failed to resolve value"
	errMsgPortInvalidAlphanumeric = "parse \"%s\": invalid port \":%s\" after host"
	errMsgPortInvalidSegmented    = "address %s: too many colons in address"
	errMsgPortMissing             = "missing port"
	errMsgUnsupportedScheme       = "missing or unsupported protocol scheme"
)

var testScenarios = []struct {
	name     string
	endpoint string

	errOTLPGRPC    error
	errMsgOTLPGRPC string

	errOTLPHTTP    error
	errMsgOTLPHTTP string

	errFluentdHTTP    error
	errMsgFluentdHTTP string
}{
	{
		name:     "with scheme: valid endpoint with path and port",
		endpoint: "https://foo.bar/foo/bar:4317",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: "",

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "empty endpoint value",
		endpoint: "",

		errOTLPGRPC:    ErrValueResolveFailed,
		errMsgOTLPGRPC: errMsgEndpointResolveFailed,

		errOTLPHTTP:    ErrValueResolveFailed,
		errMsgOTLPHTTP: errMsgEndpointResolveFailed,

		errFluentdHTTP:    ErrValueResolveFailed,
		errMsgFluentdHTTP: errMsgEndpointResolveFailed,
	},
	{
		name:     "no scheme: invalid endpoint with port",
		endpoint: "'example.com:8080'",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),
	},
	{
		name:     "with scheme: invalid endpoint with port",
		endpoint: "'https://example.com:8080'",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),
	},
	{
		name:     "no scheme: invalid endpoint",
		endpoint: "'example.com'",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: invalid endpoint",
		endpoint: "'https://example.com'",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),
	},
	{
		name:     "no scheme: missing port",
		endpoint: "example.com",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: slash port",
		endpoint: "example.com:/",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: colon port",
		endpoint: "example.com:",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: missing port",
		endpoint: "http://example.com",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: slash port",
		endpoint: "http://example.com:/",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: colon port",
		endpoint: "http://example.com:",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: valid port",
		endpoint: "example.com:8080",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: "",

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: valid port",
		endpoint: "http://example.com:8080",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: "",

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: invalid alphanumeric port",
		endpoint: "example.com:8o8o",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),
	},
	{
		name:     "with scheme: invalid alphanumeric port",
		endpoint: "http://example.com:8o8o",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),
	},
	{
		name:     "no scheme: invalid segmented port",
		endpoint: "example.com:80:80",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: invalid segmented port",
		endpoint: "http://example.com:80:80",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "https scheme: with port",
		endpoint: "https://example.com:8080",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: "",

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "https scheme: with port",
		endpoint: "https://example.com:8080",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: "",

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "https scheme: no port",
		endpoint: "https://example.com",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    nil,
		errMsgOTLPHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "grpc scheme: no port",
		endpoint: "grpc://example.com",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "random scheme: with port",
		endpoint: "rand://example.com:8080",

		errOTLPGRPC:    nil,
		errMsgOTLPGRPC: "",

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "random scheme: no port",
		endpoint: "rand://example.com",

		errOTLPGRPC:    ErrPortMissing,
		errMsgOTLPGRPC: errMsgPortMissing,

		errOTLPHTTP:    ErrUnsupportedScheme,
		errMsgOTLPHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
}

func TestOTLPGRPCEndpoints(t *testing.T) {
	for _, test := range testScenarios {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				Client: fakeClient,
			}

			err := validator.Validate(
				t.Context(),
				&telemetryv1alpha1.ValueType{Value: test.endpoint},
				OTLPProtocolGRPC)

			switch {
			case test.errOTLPGRPC != nil && test.errMsgOTLPGRPC != "":
				require.True(t, errors.Is(err, test.errOTLPGRPC))
				require.EqualError(t, err, test.errMsgOTLPGRPC)
			case test.errOTLPGRPC == nil && test.errMsgOTLPGRPC != "":
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errMsgOTLPGRPC)
			case test.errOTLPGRPC == nil:
				require.NoError(t, err)
			}
		})
	}
}

func TestOTLPHttpEndpoints(t *testing.T) {
	for _, test := range testScenarios {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				Client: fakeClient,
			}

			err := validator.Validate(
				t.Context(),
				&telemetryv1alpha1.ValueType{Value: test.endpoint},
				OTLPProtocolHTTP)

			switch {
			case test.errOTLPHTTP != nil && test.errMsgOTLPHTTP != "":
				require.True(t, errors.Is(err, test.errOTLPHTTP))
				require.EqualError(t, err, test.errMsgOTLPHTTP)
			case test.errOTLPHTTP == nil && test.errMsgOTLPHTTP != "":
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errMsgOTLPHTTP)
			case test.errOTLPHTTP == nil:
				require.NoError(t, err)
				return
			}
		})
	}
}

func TestFluentdHttpEndpoints(t *testing.T) {
	for _, test := range testScenarios {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				Client: fakeClient,
			}

			err := validator.Validate(
				t.Context(),
				&telemetryv1alpha1.ValueType{Value: test.endpoint},
				FluentdProtocolHTTP)

			switch {
			case test.errFluentdHTTP != nil && test.errMsgFluentdHTTP != "":
				require.True(t, errors.Is(err, test.errFluentdHTTP))
				require.EqualError(t, err, test.errMsgFluentdHTTP)
			case test.errFluentdHTTP == nil && test.errMsgFluentdHTTP != "":
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errMsgFluentdHTTP)
			case test.errFluentdHTTP == nil:
				require.NoError(t, err)
				return
			}
		})
	}
}

func TestMissingEndpoint(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	errNil := validator.Validate(t.Context(), nil, OTLPProtocolGRPC)
	errNoValue := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{}, OTLPProtocolGRPC)

	require.True(t, errors.Is(errNil, ErrValueResolveFailed))
	require.EqualError(t, errNil, errMsgEndpointResolveFailed)
	require.True(t, errors.Is(errNoValue, ErrValueResolveFailed))
	require.EqualError(t, errNoValue, errMsgEndpointResolveFailed)
}

func TestEndpointValueFromValid(t *testing.T) {
	validEndpoint := "http://example.com:8080"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"endpoint": []byte(validEndpoint),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(secret).Build()
	validator := Validator{
		Client: fakeClient,
	}

	errGRPC := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OTLPProtocolGRPC)
	errHTTP := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OTLPProtocolHTTP)
	errFluentd := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, FluentdProtocolHTTP)

	require.NoError(t, errGRPC)
	require.NoError(t, errHTTP)
	require.NoError(t, errFluentd)
}

func TestEndpointValueFromInvalid(t *testing.T) {
	invalidEndpoint := "'http://example.com:8080'"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"endpoint": []byte(invalidEndpoint),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(secret).Build()
	validator := Validator{
		Client: fakeClient,
	}

	errGRPC := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OTLPProtocolGRPC)
	errHTTP := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OTLPProtocolGRPC)
	errFluentd := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, FluentdProtocolHTTP)

	require.True(t, IsEndpointInvalidError(errGRPC))
	require.EqualError(t, errGRPC, fmt.Sprintf(errMsgEndpointInvalid, invalidEndpoint))
	require.True(t, IsEndpointInvalidError(errHTTP))
	require.EqualError(t, errHTTP, fmt.Sprintf(errMsgEndpointInvalid, invalidEndpoint))
	require.True(t, IsEndpointInvalidError(errFluentd))
	require.EqualError(t, errHTTP, fmt.Sprintf(errMsgEndpointInvalid, invalidEndpoint))
}

func TestEndpointValueFromMissing(t *testing.T) {
	validEndpoint := "http://example.com:8080"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"endpoint": []byte(validEndpoint),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(secret).Build()
	validator := Validator{
		Client: fakeClient,
	}

	errGRPC := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "unknown",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OTLPProtocolGRPC)
	errHTTP := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "unknown",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OTLPProtocolHTTP)
	errFluentd := validator.Validate(t.Context(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "unknown",
			Namespace: "default",
			Key:       "endpoint",
		}}}, FluentdProtocolHTTP)

	require.True(t, errors.Is(errGRPC, ErrValueResolveFailed))
	require.EqualError(t, errGRPC, errMsgEndpointResolveFailed)
	require.True(t, errors.Is(errHTTP, ErrValueResolveFailed))
	require.EqualError(t, errHTTP, errMsgEndpointResolveFailed)
	require.True(t, errors.Is(errFluentd, ErrValueResolveFailed))
	require.EqualError(t, errFluentd, errMsgEndpointResolveFailed)
}
