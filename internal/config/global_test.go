package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	g := NewGlobal()

	require.Empty(t, g.TargetNamespace())
	require.Empty(t, g.ManagerNamespace())
	require.Empty(t, g.DefaultTelemetryNamespace())
	require.False(t, g.OperateInFIPSMode())
	require.Empty(t, g.Version())
}

func TestWithNamespaces(t *testing.T) {
	g := NewGlobal(
		WithTargetNamespace("kyma-system"),
		WithManagerNamespace("kube-system"),
	)

	require.Equal(t, "kyma-system", g.TargetNamespace())
	require.Equal(t, "kube-system", g.ManagerNamespace())
	require.Equal(t, "kyma-system", g.DefaultTelemetryNamespace())
}

func TestWithOperateInFIPSMode(t *testing.T) {
	g := NewGlobal(WithOperateInFIPSMode(true))

	require.True(t, g.OperateInFIPSMode())
}

func TestWithVersion(t *testing.T) {
	g := NewGlobal(WithVersion("1.2.3"))

	require.Equal(t, "1.2.3", g.Version())
}

func TestMultipleOptions(t *testing.T) {
	g := NewGlobal(
		WithTargetNamespace("kyma-system"),
		WithOperateInFIPSMode(true),
		WithVersion("2.0.0"),
	)

	require.Equal(t, "kyma-system", g.TargetNamespace())
	require.True(t, g.OperateInFIPSMode())
	require.Equal(t, "2.0.0", g.Version())
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		shouldError bool
		expectedErr *ValidationError
	}{
		{
			name:        "valid empty namespace",
			namespace:   "",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "namespace",
				Value:   "",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "valid simple namespace",
			namespace:   "test",
			shouldError: false,
			expectedErr: nil,
		},
		{
			name:        "valid namespace with hyphen",
			namespace:   "test-namespace",
			shouldError: false,
			expectedErr: nil,
		},
		{
			name:        "valid namespace with numbers",
			namespace:   "ns123",
			shouldError: false,
			expectedErr: nil,
		},
		{
			name:        "valid single character",
			namespace:   "a",
			shouldError: false,
			expectedErr: nil,
		},
		{
			name:        "invalid namespace with spaces",
			namespace:   "invalid namespace",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "namespace",
				Value:   "invalid namespace",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace starting with hyphen",
			namespace:   "-invalid",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "namespace",
				Value:   "-invalid",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace ending with hyphen",
			namespace:   "invalid-",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "namespace",
				Value:   "invalid-",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace with uppercase",
			namespace:   "Invalid",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "namespace",
				Value:   "Invalid",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace with special characters",
			namespace:   "test@namespace",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "namespace",
				Value:   "test@namespace",
				Message: errMsgInvalidNamespace,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobal(WithTargetNamespace(tt.namespace), WithVersion("v1.0.0"))
			err := g.Validate()

			if tt.shouldError {
				require.Error(t, err)
				require.True(t, IsValidationError(err))

				var validationErr *ValidationError
				require.True(t, errors.As(err, &validationErr))
				require.Equal(t, tt.expectedErr, validationErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		shouldError bool
		expectedErr *ValidationError
	}{
		{
			name:        "valid version",
			version:     "1.2.3",
			shouldError: false,
			expectedErr: nil,
		},
		{
			name:        "empty version",
			version:     "",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "version",
				Value:   "",
				Message: errMsgEmptyOrWhitespace,
			},
		},
		{
			name:        "whitespace only version",
			version:     "   ",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "version",
				Value:   "   ",
				Message: errMsgEmptyOrWhitespace,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobal(WithTargetNamespace("test"), WithVersion(tt.version))
			err := g.Validate()

			if tt.shouldError {
				require.Error(t, err)
				require.True(t, IsValidationError(err))

				var validationErr *ValidationError
				require.True(t, errors.As(err, &validationErr))
				require.Equal(t, tt.expectedErr, validationErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	// Test that validation returns the first error encountered
	g := NewGlobal(WithTargetNamespace("Invalid@Namespace"), WithVersion(""))
	err := g.Validate()

	require.Error(t, err)
	require.True(t, IsValidationError(err))

	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	// Should return namespace error first since it's checked first
	require.Equal(t, "namespace", validationErr.Field)
	require.Equal(t, "Invalid@Namespace", validationErr.Value)
	require.Equal(t, errMsgInvalidNamespace, validationErr.Message)
}

func TestValidateSuccess(t *testing.T) {
	g := NewGlobal(
		WithTargetNamespace("kyma-system"),
		WithVersion("v1.2.3"),
		WithOperateInFIPSMode(true),
	)

	err := g.Validate()
	require.NoError(t, err)
}
