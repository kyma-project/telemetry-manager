package sharedtypes

import (
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		input    *telemetryv1alpha1.ValueType
		expected bool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty value and no ValueFrom",
			input:    &telemetryv1alpha1.ValueType{},
			expected: false,
		},
		{
			name: "non-empty value",
			input: &telemetryv1alpha1.ValueType{
				Value: "test-value",
			},
			expected: true,
		},
		{
			name: "valid ValueFrom with complete SecretKeyRef",
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
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
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: nil,
				},
			},
			expected: false,
		},
		{
			name: "ValueFrom with empty Name",
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
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
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
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
			input: &telemetryv1alpha1.ValueType{
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
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
			input: &telemetryv1alpha1.ValueType{
				Value: "test-value",
				ValueFrom: &telemetryv1alpha1.ValueFromSource{
					SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
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

func TestIsValidBeta(t *testing.T) {
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
			result := IsValidBeta(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsFilterDefined(t *testing.T) {
	tests := []struct {
		name     string
		input    []telemetryv1alpha1.FilterSpec
		expected bool
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty slice",
			input:    []telemetryv1alpha1.FilterSpec{},
			expected: false,
		},
		{
			name: "single filter",
			input: []telemetryv1alpha1.FilterSpec{
				{Conditions: []string{"condition1"}},
			},
			expected: true,
		},
		{
			name: "multiple filters",
			input: []telemetryv1alpha1.FilterSpec{
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
		input    []telemetryv1alpha1.TransformSpec
		expected bool
	}{
		{
			name:     "nil slice",
			input:    nil,
			expected: false,
		},
		{
			name:     "empty slice",
			input:    []telemetryv1alpha1.TransformSpec{},
			expected: false,
		},
		{
			name: "single transform",
			input: []telemetryv1alpha1.TransformSpec{
				{Statements: []string{"statement1"}},
			},
			expected: true,
		},
		{
			name: "multiple transforms",
			input: []telemetryv1alpha1.TransformSpec{
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
		input    *telemetryv1alpha1.OTLPInput
		expected bool
	}{
		{
			name:     "nil input defaults to enabled",
			input:    nil,
			expected: true,
		},
		{
			name: "explicitly enabled",
			input: &telemetryv1alpha1.OTLPInput{
				Disabled: false,
			},
			expected: true,
		},
		{
			name: "explicitly disabled",
			input: &telemetryv1alpha1.OTLPInput{
				Disabled: true,
			},
			expected: false,
		},
		{
			name: "enabled with namespace selector",
			input: &telemetryv1alpha1.OTLPInput{
				Disabled: false,
				Namespaces: &telemetryv1alpha1.NamespaceSelector{
					Include: []string{"namespace1"},
				},
			},
			expected: true,
		},
		{
			name: "disabled with namespace selector",
			input: &telemetryv1alpha1.OTLPInput{
				Disabled: true,
				Namespaces: &telemetryv1alpha1.NamespaceSelector{
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
