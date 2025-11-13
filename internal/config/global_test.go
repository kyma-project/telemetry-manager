package config

import (
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
	g := NewGlobal(WithVersion("v1.2.3"))

	require.Equal(t, "v1.2.3", g.Version())
}

func TestMultipleOptions(t *testing.T) {
	g := NewGlobal(
		WithNamespace("kyma-system"),
		WithOperateInFIPSMode(true),
		WithVersion("v2.0.0"),
	)

	require.Equal(t, "kyma-system", g.TargetNamespace())
	require.True(t, g.OperateInFIPSMode())
	require.Equal(t, "v2.0.0", g.Version())
}
