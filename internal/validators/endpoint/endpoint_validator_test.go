package endpoint

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

const (
	endpointValid            = "http://example.com"
	endpointInvalid          = "'http://example.com'"
	endpointWithPortValid1   = "http://example.com:8080"
	endpointWithPortValid2   = "example.com:8080"
	endpointWithPortMissing  = "http://example.com:/"
	endpointWithPortInvalid1 = "http://example.com:9ab2"
	endpointWithPortInvalid2 = "http://example.com:80:80"
	endpointWithPortInvalid3 = "example.com:9ab2"
	endpointWithPortInvalid4 = "example.com:80:80"

	endpointMissingErrMessage = "failed to resolve value"
	endpointInvalidErrMessage = "parse \"'http://example.com'\": first path segment in URL cannot contain colon"
	portMissingErrMessage     = "missing port"
	portInvalid1ErrMessage    = "parse \"http://example.com:9ab2\": invalid port \":9ab2\" after host"
	portInvalid2ErrMessage    = "address example.com:80:80: too many colons in address"
	portInvalid3ErrMessage    = "parse \"example.com:9ab2\": invalid port \":9ab2\" after host"
	portInvalid4ErrMessage    = "address example.com:80:80: too many colons in address"
)

func TestMissingEndpoint(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), nil, false)

	require.True(t, errors.Is(err, ErrValueResolveFailed))
	require.EqualError(t, err, endpointMissingErrMessage)
}

func TestEmptyEndpoint(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{}, false)

	require.True(t, errors.Is(err, ErrValueResolveFailed))
	require.EqualError(t, err, endpointMissingErrMessage)
}

func TestEndpointWithPortMissing(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortMissing}, true)

	require.True(t, errors.Is(err, ErrPortMissing))
}

func TestEndpointValueValid(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointValid}, false)

	require.NoError(t, err)
}

func TestEndpointValueWithPortValid(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err1 := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortValid1}, true)
	err2 := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortValid2}, true)

	require.NoError(t, err1)
	require.NoError(t, err2)
}

func TestEndpointValueInvalid(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointInvalid}, false)

	require.True(t, IsEndpointInvalidError(err))
	require.EqualError(t, err, endpointInvalidErrMessage)
}

func TestEndpointWithPortInvalid(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err1 := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortInvalid1}, true)
	err2 := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortInvalid2}, true)
	err3 := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortInvalid3}, true)
	err4 := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortInvalid4}, true)

	require.True(t, IsEndpointInvalidError(err1))
	require.EqualError(t, err1, portInvalid1ErrMessage)
	require.True(t, IsEndpointInvalidError(err2))
	require.EqualError(t, err2, portInvalid2ErrMessage)
	require.True(t, IsEndpointInvalidError(err3))
	require.EqualError(t, err3, portInvalid3ErrMessage)
	require.True(t, IsEndpointInvalidError(err4))
	require.EqualError(t, err4, portInvalid4ErrMessage)
}

func TestEndpointValueFromValid(t *testing.T) {
	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"endpoint": []byte(endpointValid),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(validSecret).Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, false)

	require.NoError(t, err)
}

func TestEndpointValueFromInvalid(t *testing.T) {
	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"endpoint": []byte(endpointInvalid),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(validSecret).Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "test",
			Namespace: "default",
			Key:       "endpoint",
		}}}, false)

	require.True(t, IsEndpointInvalidError(err))
	require.EqualError(t, err, endpointInvalidErrMessage)
}

func TestEndpointValueFromMissing(t *testing.T) {
	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"endpoint": []byte(endpointInvalid),
		},
	}

	fakeClient := fake.NewClientBuilder().WithObjects(validSecret).Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.TODO(), &telemetryv1alpha1.ValueType{ValueFrom: &telemetryv1alpha1.ValueFromSource{
		SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
			Name:      "unknown",
			Namespace: "default",
			Key:       "endpoint",
		}}}, false)

	require.True(t, errors.Is(err, ErrValueResolveFailed))
	require.EqualError(t, err, endpointMissingErrMessage)
}
