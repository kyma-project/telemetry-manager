package lokilogpipeline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeLokiLogPipeline(t *testing.T) {
	lokiLogPipeline := MakeLokiLogPipeline()

	require.NotNil(t, lokiLogPipeline)
	require.Equal(t, "telemetry.kyma-project.io/v1alpha1", lokiLogPipeline.APIVersion)
	require.Equal(t, "LogPipeline", lokiLogPipeline.Kind)
	require.Equal(t, "loki", lokiLogPipeline.Name)
	require.Equal(t, true, lokiLogPipeline.Spec.Input.Application.Namespaces.System)
	require.Equal(t, "http://logging-loki:3100/loki/api/v1/push", lokiLogPipeline.Spec.Output.Loki.URL.Value)
	require.Equal(t, map[string]string{"job": "telemetry-fluent-bit"}, lokiLogPipeline.Spec.Output.Loki.Labels)
	require.Equal(t, []string{"kubernetes", "stream"}, lokiLogPipeline.Spec.Output.Loki.RemoveKeys)
}
