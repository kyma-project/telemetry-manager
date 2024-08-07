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
	endpointValid             = "http://example.com"
	endpointInvalid           = "'http://example.com'"
	endpointMissingErrMessage = "either value or secret key reference must be provided"
	endpointInvalidErrMessage = "parse \"'http://example.com'\": first path segment in URL cannot contain colon"
)

func TestMissingEndpoint(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), nil)

	var endpointInvalidError *EndpointInvalidError
	require.True(t, errors.As(err, &endpointInvalidError))
	require.EqualError(t, err, endpointMissingErrMessage)
}

func TestEmptyEndpoint(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{})

	var endpointInvalidError *EndpointInvalidError
	require.True(t, errors.As(err, &endpointInvalidError))
	require.EqualError(t, err, endpointMissingErrMessage)
}

func TestEndpointValueValid(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointValid})

	require.NoError(t, err)
}

func TestEndpointValueInvalid(t *testing.T) {
	fakeClient := fake.NewClientBuilder().Build()
	validator := Validator{
		Client: fakeClient,
	}

	err := validator.Validate(context.Background(), &telemetryv1alpha1.ValueType{Value: endpointInvalid})

	var endpointInvalidError *EndpointInvalidError
	require.True(t, errors.As(err, &endpointInvalidError))
	require.EqualError(t, err, endpointInvalidErrMessage)
}

func TestEndpointValueFromValid(t *testing.T) {
	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"apikey": []byte(endpointValid),
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
			Key:       "apikey",
		}}})

	require.NoError(t, err)
}

func TestEndpointValueFromInvalid(t *testing.T) {
	validSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"apikey": []byte(endpointInvalid),
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
			Key:       "apikey",
		}}})

	var endpointInvalidError *EndpointInvalidError
	require.True(t, errors.As(err, &endpointInvalidError))
	require.EqualError(t, err, endpointInvalidErrMessage)
}
