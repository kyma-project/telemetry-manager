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
		WithVersion("2.0.0"),
		WithOperateInFIPSMode(true),
	)

	require.Equal(t, "kyma-system", g.TargetNamespace())
	require.True(t, g.OperateInFIPSMode())
	require.Equal(t, "2.0.0", g.Version())
}

func TestValidateTargetNamespace(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		shouldError bool
		expectedErr *ValidationError
	}{
		{
			name:        "empty target namespace",
			namespace:   "",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "target_namespace",
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
				Field:   "target_namespace",
				Value:   "invalid namespace",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace starting with hyphen",
			namespace:   "-invalid",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "target_namespace",
				Value:   "-invalid",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace ending with hyphen",
			namespace:   "invalid-",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "target_namespace",
				Value:   "invalid-",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace with uppercase",
			namespace:   "Invalid",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "target_namespace",
				Value:   "Invalid",
				Message: errMsgInvalidNamespace,
			},
		},
		{
			name:        "invalid namespace with special characters",
			namespace:   "test@namespace",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "target_namespace",
				Value:   "test@namespace",
				Message: errMsgInvalidNamespace,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobal(
				WithTargetNamespace(tt.namespace),
				WithManagerNamespace("valid-manager"),
				WithVersion("v1.0.0"),
			)
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

func TestValidateManagerNamespace(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		shouldError bool
		expectedErr *ValidationError
	}{
		{
			name:        "empty manager namespace",
			namespace:   "",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "manager_namespace",
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
			name:        "invalid namespace with uppercase",
			namespace:   "Invalid",
			shouldError: true,
			expectedErr: &ValidationError{
				Field:   "manager_namespace",
				Value:   "Invalid",
				Message: errMsgInvalidNamespace,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGlobal(
				WithTargetNamespace("valid-target"),
				WithManagerNamespace(tt.namespace),
				WithVersion("v1.0.0"),
			)
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
			g := NewGlobal(
				WithTargetNamespace("valid-target"),
				WithManagerNamespace("valid-manager"),
				WithVersion(tt.version),
			)
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
	// Test that validation returns the first error encountered (target namespace is checked first)
	g := NewGlobal(
		WithTargetNamespace("Invalid@Namespace"),
		WithManagerNamespace("Invalid@Manager"),
		WithVersion(""),
	)
	err := g.Validate()

	require.Error(t, err)
	require.True(t, IsValidationError(err))

	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	// Should return target namespace error first since it's checked first
	require.Equal(t, "target_namespace", validationErr.Field)
	require.Equal(t, "Invalid@Namespace", validationErr.Value)
	require.Equal(t, errMsgInvalidNamespace, validationErr.Message)
}

func TestValidateSuccess(t *testing.T) {
	g := NewGlobal(
		WithTargetNamespace("kyma-system"),
		WithManagerNamespace("kube-system"),
		WithVersion("v1.2.3"),
		WithOperateInFIPSMode(true),
	)

	err := g.Validate()
	require.NoError(t, err)
}

func TestGettersForOptionalFields(t *testing.T) {
	labels := map[string]string{"l1": "v1"}
	annotations := map[string]string{"a1": "v1"}
	g := NewGlobal(
		WithImagePullSecretName("my-secret"),
		WithClusterTrustBundleName("trust-bundle"),
		WithAdditionalLabels(labels),
		WithAdditionalAnnotations(annotations),
	)

	require.Equal(t, "my-secret", g.ImagePullSecretName())
	require.Equal(t, "trust-bundle", g.ClusterTrustBundleName())
	require.Equal(t, labels, g.AdditionalLabels())
	require.Equal(t, annotations, g.AdditionalAnnotations())
}
