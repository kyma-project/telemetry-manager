package sharedtypes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1beta1.ValueType
		expected bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty value and no ValueFrom",
			input:    &telemetryv1beta1.ValueType{},
			expected: false,
		},
		{
			name: "non-empty value",
			input: &telemetryv1beta1.ValueType{
				Value: "test-value",
			},
			expected: true,
		},
		{
			name: "valid ValueFrom with complete SecretKeyRef",
			input: &telemetryv1beta1.ValueType{
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
						Name:      "secret-name",
						Key:       "secret-key",
						Namespace: "secret-namespace",
					},
				},
			},
			expected: true,
		},
		{
			name: "ValueFrom with nil SecretKeyRef",
			input: &telemetryv1beta1.ValueType{
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: nil,
				},
			},
			expected: false,
		},
		{
			name: "ValueFrom with empty Name",
			input: &telemetryv1beta1.ValueType{
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
						Name:      "",
						Key:       "secret-key",
						Namespace: "secret-namespace",
					},
				},
			},
			expected: false,
		},
		{
			name: "ValueFrom with empty Key",
			input: &telemetryv1beta1.ValueType{
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
						Name:      "secret-name",
						Key:       "",
						Namespace: "secret-namespace",
					},
				},
			},
			expected: false,
		},
		{
			name: "ValueFrom with empty Namespace",
			input: &telemetryv1beta1.ValueType{
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
						Name:      "secret-name",
						Key:       "secret-key",
						Namespace: "",
					},
				},
			},
			expected: false,
		},
		{
			name: "both Value and ValueFrom present",
			input: &telemetryv1beta1.ValueType{
				Value: "test-value",
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
						Name:      "secret-name",
						Key:       "secret-key",
						Namespace: "secret-namespace",
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValid(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveValue(t *testing.T) {
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
		value       telemetryv1beta1.ValueType
		expected    []byte
		expectedErr bool
		errType     error
	}{
		{
			name:        "resolve from direct value",
			value:       telemetryv1beta1.ValueType{Value: "direct-value"},
			expected:    []byte("direct-value"),
			expectedErr: false,
		},
		{
			name: "resolve from undefined value",
			value: telemetryv1beta1.ValueType{
				Value: "",
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: nil,
				},
			},
			expected:    nil,
			expectedErr: true,
			errType:     ErrValueOrSecretRefUndefined,
		},
		{
			name: "resolve from secret",
			value: telemetryv1beta1.ValueType{
				Value: "",
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
			value: telemetryv1beta1.ValueType{
				Value: "direct-value",
				ValueFrom: &telemetryv1beta1.ValueFromSource{
					SecretKeyRef: &telemetryv1beta1.SecretKeyRef{
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
			result, err := ResolveValue(t.Context(), client, tt.value)
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

func TestIsFilterDefined(t *testing.T) {
	tests := []struct {
		name     string
		input    []telemetryv1beta1.FilterSpec
		expected bool
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty slice",
			input:    []telemetryv1beta1.FilterSpec{},
			expected: false,
		},
		{
			name: "single filter",
			input: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"condition1"}},
			},
			expected: true,
		},
		{
			name: "multiple filters",
			input: []telemetryv1beta1.FilterSpec{
				{Conditions: []string{"condition1"}},
				{Conditions: []string{"condition2"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFilterDefined(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsTransformDefined(t *testing.T) {
	tests := []struct {
		name     string
		input    []telemetryv1beta1.TransformSpec
		expected bool
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty slice",
			input:    []telemetryv1beta1.TransformSpec{},
			expected: false,
		},
		{
			name: "single transform",
			input: []telemetryv1beta1.TransformSpec{
				{Statements: []string{"statement1"}},
			},
			expected: true,
		},
		{
			name: "multiple transforms",
			input: []telemetryv1beta1.TransformSpec{
				{Statements: []string{"statement1"}},
				{Statements: []string{"statement2"}},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransformDefined(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsOTLPInputEnabled(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1beta1.OTLPInput
		expected bool
	}{
		{
			name:     "nil input defaults to enabled",
			input:    nil,
			expected: true,
		},
		{
			name: "explicitly enabled",
			input: &telemetryv1beta1.OTLPInput{
				Enabled: ptr.To(true),
			},
			expected: true,
		},
		{
			name: "explicitly disabled",
			input: &telemetryv1beta1.OTLPInput{
				Enabled: ptr.To(false),
			},
			expected: false,
		},
		{
			name: "enabled with namespace selector",
			input: &telemetryv1beta1.OTLPInput{
				Enabled: ptr.To(true),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"namespace1"},
				},
			},
			expected: true,
		},
		{
			name: "disabled with namespace selector",
			input: &telemetryv1beta1.OTLPInput{
				Enabled: ptr.To(false),
				Namespaces: &telemetryv1beta1.NamespaceSelector{
					Include: []string{"namespace1"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsOTLPInputEnabled(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}
