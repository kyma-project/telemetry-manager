package sharedtypes

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1alpha1.ValueType
		expected bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: false,
		},
		{
			name:     "non-empty value field",
			input:    &telemetryv1alpha1.ValueType{Value: "test"},
			expected: true,
		},
		{
			name:     "empty value and nil ValueFrom",
			input:    &telemetryv1alpha1.ValueType{Value: ""},
			expected: false,
		},
		{
			name: "valid SecretKeyRef",
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      "secret",
						Key:       "key",
						Namespace: "default",
					},
				},
			},
			expected: true,
		},
		{
			name: "SecretKeyRef missing name",
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Key:       "key",
						Namespace: "default",
					},
				},
			},
			expected: false,
		},
		{
			name: "SecretKeyRef missing key",
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      "secret",
						Namespace: "default",
					},
				},
			},
			expected: false,
		},
		{
			name: "SecretKeyRef missing namespace",
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name: "secret",
						Key:  "key",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValid(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveValue(t *testing.T) {
	ctx := context.Background()

	testSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"test-key": []byte("secret-value"),
		},
	}

	client := fake.NewClientBuilder().WithObjects(&testSecret).Build()

	tests := []struct {
		name        string
		value       telemetryv1alpha1.ValueType
		expected    []byte
		expectedErr bool
		errType     error
	}{
		{
			name:        "resolve from direct value",
			value:       telemetryv1alpha1.ValueType{Value: "direct-value"},
			expected:    []byte("direct-value"),
			expectedErr: false,
		},
		{
			name: "resolve from undefined value",
			value: telemetryv1alpha1.ValueType{
				Value: "",
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: nil,
				},
			},
			expected:    nil,
			expectedErr: true,
			errType:     ErrValueOrSecretRefUndefined,
		},
		{
			name: "resolve from secret",
			value: telemetryv1alpha1.ValueType{
				Value: "",
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      "test-secret",
						Key:       "test-key",
						Namespace: "default",
					},
				},
			},
			expected:    []byte("secret-value"),
			expectedErr: false,
		},
		{
			name: "direct value takes precedence over secret",
			value: telemetryv1alpha1.ValueType{
				Value: "direct-value",
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
						Name:      "test-secret",
						Key:       "test-key",
						Namespace: "default",
					},
				},
			},
			expected:    []byte("direct-value"),
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveValue(ctx, client, tt.value)
			if tt.expectedErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, tt.errType))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
