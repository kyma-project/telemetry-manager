package envvar

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatVariableName(t *testing.T) {
	expected := "PIPELINE_TEST_NAMESPACE_TEST_NAME_TEST_KEY_123"
	actual := FormatEnvVarName("pipeline", "test-namespace", "test-name", "TEST_KEY_123")
	require.Equal(t, expected, actual)
}

func TestGenerateVariableNameFromLowercase(t *testing.T) {
	expected := "PIPELINE_TEST_NAMESPACE_TEST_NAME_TEST_KEY_123"
	actual := FormatEnvVarName("pipeline", "test-namespace", "test-name", "test-key.123")
	require.Equal(t, expected, actual)
}
