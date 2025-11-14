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

func TestWithNamespace(t *testing.T) {
	g := NewGlobal(WithNamespace("test-namespace"))

	require.Equal(t, "test-namespace", g.TargetNamespace())
	require.Equal(t, "test-namespace", g.ManagerNamespace())
	require.Equal(t, "test-namespace", g.DefaultTelemetryNamespace())
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
		WithNamespace("kyma-system"),
		WithOperateInFIPSMode(true),
		WithVersion("2.0.0"),
	)

	require.Equal(t, "kyma-system", g.TargetNamespace())
	require.True(t, g.OperateInFIPSMode())
	require.Equal(t, "2.0.0", g.Version())
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name          string
		namespace     string
		shouldError   bool
		expectedField string
		expectedValue string
		expectedMsg   string
	}{
		{
			name:          "valid empty namespace",
			namespace:     "",
			shouldError:   true,
			expectedField: "namespace",
			expectedValue: "",
			expectedMsg:   "must be a valid Kubernetes namespace name",
		},
		{
			name:        "valid simple namespace",
			namespace:   "test",
			shouldError: false,
		},
		{
			name:        "valid namespace with hyphen",
			namespace:   "test-namespace",
			shouldError: false,
		},
		{
			name:        "valid namespace with numbers",
			namespace:   "ns123",
			shouldError: false,
		},
		{
			name:        "valid single character",
			namespace:   "a",
			shouldError: false,
		},
		{
			name:          "invalid namespace with spaces",
			namespace:     "invalid namespace",
			shouldError:   true,
			expectedField: "namespace",
			expectedValue: "invalid namespace",
			expectedMsg:   "must be a valid Kubernetes namespace name",
		},
		{
			name:          "invalid namespace starting with hyphen",
			namespace:     "-invalid",
			shouldError:   true,
			expectedField: "namespace",
			expectedValue: "-invalid",
			expectedMsg:   "must be a valid Kubernetes namespace name",
		},
		{
			name:          "invalid namespace ending with hyphen",
			namespace:     "invalid-",
			shouldError:   true,
			expectedField: "namespace",
			expectedValue: "invalid-",
			expectedMsg:   "must be a valid Kubernetes namespace name",
		},
		{
			name:          "invalid namespace with uppercase",
			namespace:     "Invalid",
			shouldError:   true,
			expectedField: "namespace",
			expectedValue: "Invalid",
			expectedMsg:   "must be a valid Kubernetes namespace name",
		},
		{
			name:          "invalid namespace with special characters",
			namespace:     "test@namespace",
			shouldError:   true,
			expectedField: "namespace",
			expectedValue: "test@namespace",
			expectedMsg:   "must be a valid Kubernetes namespace name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobal(WithNamespace(tt.namespace), WithVersion("v1.0.0"))
			err := g.Validate()

			if tt.shouldError {
				require.Error(t, err)
				require.True(t, IsValidationError(err))

				var validationErr *ValidationError
				require.True(t, errors.As(err, &validationErr))
				require.Equal(t, tt.expectedField, validationErr.Field)
				require.Equal(t, tt.expectedValue, validationErr.Value)
				require.Equal(t, tt.expectedMsg, validationErr.Message)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		shouldError   bool
		expectedField string
		expectedValue string
		expectedMsg   string
	}{
		{
			name:        "valid version",
			version:     "1.2.3",
			shouldError: false,
		},
		{
			name:        "valid complex version",
			version:     "2.0.0-rc.1",
			shouldError: false,
		},
		{
			name:          "empty version",
			version:       "",
			shouldError:   true,
			expectedField: "version",
			expectedValue: "",
			expectedMsg:   "cannot be empty or whitespace only",
		},
		{
			name:          "whitespace only version",
			version:       "   ",
			shouldError:   true,
			expectedField: "version",
			expectedValue: "   ",
			expectedMsg:   "cannot be empty or whitespace only",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobal(WithNamespace("test"), WithVersion(tt.version))
			err := g.Validate()

			if tt.shouldError {
				require.Error(t, err)
				require.True(t, IsValidationError(err))

				var validationErr *ValidationError
				require.True(t, errors.As(err, &validationErr))
				require.Equal(t, tt.expectedField, validationErr.Field)
				require.Equal(t, tt.expectedValue, validationErr.Value)
				require.Equal(t, tt.expectedMsg, validationErr.Message)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	// Test that validation returns the first error encountered
	g := NewGlobal(WithNamespace("Invalid@Namespace"), WithVersion(""))
	err := g.Validate()

	require.Error(t, err)
	require.True(t, IsValidationError(err))

	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	// Should return namespace error first since it's checked first
	require.Equal(t, "namespace", validationErr.Field)
	require.Equal(t, "Invalid@Namespace", validationErr.Value)
	require.Equal(t, "must be a valid Kubernetes namespace name", validationErr.Message)
}

func TestValidateSuccess(t *testing.T) {
	g := NewGlobal(
		WithNamespace("kyma-system"),
		WithVersion("v1.2.3"),
		WithOperateInFIPSMode(true),
	)

	err := g.Validate()
	require.NoError(t, err)
}
