package endpoint

import (
	"context"
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

	errOtlpGRPC    error
	errMsgOtlpGRPC string

	errOtlpHTTP    error
	errMsgOtlpHTTP string

	errFluentdHTTP    error
	errMsgFluentdHTTP string
}{
	{
		name:     "empty endpoint value",
		endpoint: "",

		errOtlpGRPC:    ErrValueResolveFailed,
		errMsgOtlpGRPC: errMsgEndpointResolveFailed,

		errOtlpHTTP:    ErrValueResolveFailed,
		errMsgOtlpHTTP: errMsgEndpointResolveFailed,

		errFluentdHTTP:    ErrValueResolveFailed,
		errMsgFluentdHTTP: errMsgEndpointResolveFailed,
	},
	{
		name:     "no scheme: invalid endpoint with port",
		endpoint: "'example.com:8080'",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),
	},
	{
		name:     "with scheme: invalid endpoint with port",
		endpoint: "'https://example.com:8080'",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),
	},
	{
		name:     "no scheme: invalid endpoint",
		endpoint: "'example.com'",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: invalid endpoint",
		endpoint: "'https://example.com'",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),
	},
	{
		name:     "no scheme: missing port",
		endpoint: "example.com",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: slash port",
		endpoint: "example.com:/",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: colon port",
		endpoint: "example.com:",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: missing port",
		endpoint: "http://example.com",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: slash port",
		endpoint: "http://example.com:/",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: colon port",
		endpoint: "http://example.com:",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: valid port",
		endpoint: "example.com:8080",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: "",

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: valid port",
		endpoint: "http://example.com:8080",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: "",

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "no scheme: invalid alphanumeric port",
		endpoint: "example.com:8o8o",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),
	},
	{
		name:     "with scheme: invalid alphanumeric port",
		endpoint: "http://example.com:8o8o",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),
	},
	{
		name:     "no scheme: invalid segmented port",
		endpoint: "example.com:80:80",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "with scheme: invalid segmented port",
		endpoint: "http://example.com:80:80",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "https scheme: with port",
		endpoint: "https://example.com:8080",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: "",

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "https scheme: with port",
		endpoint: "https://example.com:8080",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: "",

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "https scheme: no port",
		endpoint: "https://example.com",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    nil,
		errMsgOtlpHTTP: "",

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "grpc scheme: no port",
		endpoint: "grpc://example.com",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "random scheme: with port",
		endpoint: "rand://example.com:8080",

		errOtlpGRPC:    nil,
		errMsgOtlpGRPC: "",

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
	{
		name:     "random scheme: no port",
		endpoint: "rand://example.com",

		errOtlpGRPC:    ErrPortMissing,
		errMsgOtlpGRPC: errMsgPortMissing,

		errOtlpHTTP:    ErrUnsupportedScheme,
		errMsgOtlpHTTP: errMsgUnsupportedScheme,

		errFluentdHTTP:    nil,
		errMsgFluentdHTTP: "",
	},
}

func TestOtlpGrpcEndpoints(t *testing.T) {
	for _, test := range testScenarios {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				Client: fakeClient,
			}

			err := validator.Validate(
				context.Background(),
				&telemetryv1alpha1.ValueType{Value: test.endpoint},
				OtlpProtocolGRPC)

			if test.errOtlpGRPC != nil && test.errMsgOtlpGRPC != "" {
				require.True(t, errors.Is(err, test.errOtlpGRPC))
				require.EqualError(t, err, test.errMsgOtlpGRPC)
			} else if test.errOtlpGRPC == nil && test.errMsgOtlpGRPC != "" {
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errMsgOtlpGRPC)
			} else if test.errOtlpGRPC == nil {
				require.NoError(t, err)
				return
			}
		})
	}
}

func TestOtlpHttpEndpoints(t *testing.T) {
	for _, test := range testScenarios {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().Build()
			validator := Validator{
				Client: fakeClient,
			}

			err := validator.Validate(
				context.Background(),
				&telemetryv1alpha1.ValueType{Value: test.endpoint},
				OtlpProtocolHTTP)

			if test.errOtlpHTTP != nil && test.errMsgOtlpHTTP != "" {
				require.True(t, errors.Is(err, test.errOtlpHTTP))
				require.EqualError(t, err, test.errMsgOtlpHTTP)
			} else if test.errOtlpHTTP == nil && test.errMsgOtlpHTTP != "" {
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errMsgOtlpHTTP)
			} else if test.errOtlpHTTP == nil {
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
				context.Background(),
				&telemetryv1alpha1.ValueType{Value: test.endpoint},
				FluentdProtocolHTTP)

			if test.errFluentdHTTP != nil && test.errMsgFluentdHTTP != "" {
				require.True(t, errors.Is(err, test.errFluentdHTTP))
				require.EqualError(t, err, test.errMsgFluentdHTTP)
			} else if test.errFluentdHTTP == nil && test.errMsgFluentdHTTP != "" {
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errMsgFluentdHTTP)
			} else if test.errFluentdHTTP == nil {
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

	errNil := validator.Validate(context.Background(), nil, OtlpProtocolGRPC)
	errNoValue := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{}, OtlpProtocolGRPC)

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

	errGRPC := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OtlpProtocolGRPC)
	errHTTP := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OtlpProtocolHTTP)
	errFluentd := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
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

	errGRPC := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OtlpProtocolGRPC)
	errHTTP := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OtlpProtocolGRPC)
	errFluentd := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
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

	errGRPC := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "unknown",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OtlpProtocolGRPC)
	errHTTP := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "unknown",
			Namespace: "default",
			Key:       "endpoint",
		}}}, OtlpProtocolHTTP)
	errFluentd := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
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
