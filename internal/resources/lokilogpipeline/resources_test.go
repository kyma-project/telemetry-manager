package lokilogpipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeLokiLogPipeline(t *testing.T) {
	lokiLogPipeline := MakeLokiLogPipeline()

	require.NotNil(t, lokiLogPipeline)
	require.Equal(t, lokiLogPipeline.APIVersion, "telemetry.kyma-project.io/v1alpha1")
	require.Equal(t, lokiLogPipeline.Kind, "LogPipeline")
	require.Equal(t, lokiLogPipeline.Name, "loki")
	require.Equal(t, lokiLogPipeline.Spec.Input.Application.Namespaces.System, true)
	require.Equal(t, lokiLogPipeline.Spec.Output.Loki.URL.Value, "http://logging-loki:3100/loki/api/v1/push")
	require.Equal(t, lokiLogPipeline.Spec.Output.Loki.Labels, map[string]string{"job": "telemetry-fluent-bit"})
	require.Equal(t, lokiLogPipeline.Spec.Output.Loki.RemoveKeys, []string{"kubernetes", "stream"})
}
