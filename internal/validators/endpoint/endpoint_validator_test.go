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
	endpointValid           = "http://example.com"
	endpointInvalid         = "'http://example.com'"
	endpointWithPortValid   = "http://example.com:8080"
	endpointWithPortMissing = "http://example.com:/"
	endpointWithPortInvalid = "http://example.com:9abc2"

	endpointMissingErrMessage = "failed to resolve value"
	endpointInvalidErrMessage = "parse \"'http://example.com'\": first path segment in URL cannot contain colon"
	portMissingErrMessage     = "missing port"
	portInvalidErrMessage     = "parse \"http://example.com:9abc2\": invalid port \":9abc2\" after host"
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

	require.True(t, errors.Is(err, ErrMissingPort))
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

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortValid}, true)

	require.NoError(t, err)
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

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointWithPortInvalid}, true)

	require.True(t, IsEndpointInvalidError(err))
	require.EqualError(t, err, portInvalidErrMessage)
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
