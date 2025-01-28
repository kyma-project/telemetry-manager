package agent

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMakeExtension(t *testing.T) {
	ext := makeExtensionsConfig()
	require.Equal(t, "/var/lib/otel", ext.FileStorage.Directory)
	require.Equal(t, "${MY_POD_IP}:13133", ext.HealthCheck.Endpoint)
	require.Equal(t, "127.0.0.1:1777", ext.Pprof.Endpoint)
}
