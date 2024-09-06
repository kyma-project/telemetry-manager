package endpoint

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	errOtlpGrpc    error
	errmsgOtlpGrpc string

	errOtlpHttp    error
	errmsgOtlpHttp string

	errFluentdHttp    error
	errmsgFluentdHttp string
}{
	{
		name:     "empty endpoint value",
		endpoint: "",

		errOtlpGrpc:    ErrValueResolveFailed,
		errmsgOtlpGrpc: errMsgEndpointResolveFailed,

		errOtlpHttp:    ErrValueResolveFailed,
		errmsgOtlpHttp: errMsgEndpointResolveFailed,

		errFluentdHttp:    ErrValueResolveFailed,
		errmsgFluentdHttp: errMsgEndpointResolveFailed,
	},
	{
		name:     "no scheme: invalid endpoint with port",
		endpoint: "'example.com:8080'",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: fmt.Sprintf(errMsgEndpointInvalid, "'example.com:8080'"),
	},
	{
		name:     "with scheme: invalid endpoint with port",
		endpoint: "'https://example.com:8080'",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com:8080'"),
	},
	{
		name:     "no scheme: invalid endpoint",
		endpoint: "'example.com'",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "with scheme: invalid endpoint",
		endpoint: "'https://example.com'",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: fmt.Sprintf(errMsgEndpointInvalid, "'https://example.com'"),
	},
	{
		name:     "no scheme: missing port",
		endpoint: "example.com",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "no scheme: slash port",
		endpoint: "example.com:/",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "no scheme: colon port",
		endpoint: "example.com:",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "with scheme: missing port",
		endpoint: "http://example.com",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "with scheme: slash port",
		endpoint: "http://example.com:/",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "with scheme: colon port",
		endpoint: "http://example.com:",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "no scheme: valid port",
		endpoint: "example.com:8080",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: "",

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "with scheme: valid port",
		endpoint: "http://example.com:8080",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: "",

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "no scheme: invalid alphanumeric port",
		endpoint: "example.com:8o8o",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "example.com:8o8o", "8o8o"),
	},
	{
		name:     "with scheme: invalid alphanumeric port",
		endpoint: "http://example.com:8o8o",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: fmt.Sprintf(errMsgPortInvalidAlphanumeric, "http://example.com:8o8o", "8o8o"),
	},
	{
		name:     "no scheme: invalid segmented port",
		endpoint: "example.com:80:80",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "with scheme: invalid segmented port",
		endpoint: "http://example.com:80:80",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errOtlpHttp:    nil,
		errmsgOtlpHttp: fmt.Sprintf(errMsgPortInvalidSegmented, "example.com:80:80"),

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "https scheme: with port",
		endpoint: "https://example.com:8080",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: "",

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "https scheme: with port",
		endpoint: "https://example.com:8080",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: "",

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "https scheme: no port",
		endpoint: "https://example.com",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    nil,
		errmsgOtlpHttp: "",

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "grpc scheme: no port",
		endpoint: "grpc://example.com",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "random scheme: with port",
		endpoint: "rand://example.com:8080",

		errOtlpGrpc:    nil,
		errmsgOtlpGrpc: "",

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
	},
	{
		name:     "random scheme: no port",
		endpoint: "rand://example.com",

		errOtlpGrpc:    ErrPortMissing,
		errmsgOtlpGrpc: errMsgPortMissing,

		errOtlpHttp:    ErrUnsupportedScheme,
		errmsgOtlpHttp: errMsgUnsupportedScheme,

		errFluentdHttp:    nil,
		errmsgFluentdHttp: "",
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

			if test.errOtlpGrpc != nil && test.errmsgOtlpGrpc != "" {
				require.True(t, errors.Is(err, test.errOtlpGrpc))
				require.EqualError(t, err, test.errmsgOtlpGrpc)
			} else if test.errOtlpGrpc == nil && test.errmsgOtlpGrpc != "" {
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errmsgOtlpGrpc)
			} else if test.errOtlpGrpc == nil {
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

			if test.errOtlpHttp != nil && test.errmsgOtlpHttp != "" {
				require.True(t, errors.Is(err, test.errOtlpHttp))
				require.EqualError(t, err, test.errmsgOtlpHttp)
			} else if test.errOtlpHttp == nil && test.errmsgOtlpHttp != "" {
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errmsgOtlpHttp)
			} else if test.errOtlpHttp == nil {
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

			if test.errFluentdHttp != nil && test.errmsgFluentdHttp != "" {
				require.True(t, errors.Is(err, test.errFluentdHttp))
				require.EqualError(t, err, test.errmsgFluentdHttp)
			} else if test.errFluentdHttp == nil && test.errmsgFluentdHttp != "" {
				require.True(t, IsEndpointInvalidError(err))
				require.EqualError(t, err, test.errmsgFluentdHttp)
			} else if test.errFluentdHttp == nil {
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
