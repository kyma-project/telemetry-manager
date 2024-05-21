package conditions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMessageFor(t *testing.T) {
	t.Run("should return correct message which is common to all pipelines", func(t *testing.T) {
		message := MessageForLogPipeline(ReasonReferencedSecretMissing)
		require.Equal(t, commonMessages[ReasonReferencedSecretMissing], message)
	})

	t.Run("should return correct message which is unique to each pipeline", func(t *testing.T) {
		logsDaemonSetNotReadyMessage := MessageForLogPipeline(ReasonAgentNotReady)
		require.Equal(t, logPipelineMessages[ReasonAgentNotReady], logsDaemonSetNotReadyMessage)

		tracesDeploymentNotReadyMessage := MessageForTracePipeline(ReasonGatewayNotReady)
		require.Equal(t, tracePipelineMessages[ReasonGatewayNotReady], tracesDeploymentNotReadyMessage)

		metricsDeploymentNotReadyMessage := MessageForMetricPipeline(ReasonGatewayNotReady)
		require.Equal(t, metricPipelineMessages[ReasonGatewayNotReady], metricsDeploymentNotReadyMessage)
	})

	t.Run("should return empty message for reasons which do not have a specialized message", func(t *testing.T) {
		metricsAgentNotRequiredMessage := MessageForMetricPipeline(ReasonMetricAgentNotRequired)
		require.Equal(t, "", metricsAgentNotRequiredMessage)
	})
}
