package runtimemetrics

import (
	"testing"

	"github.com/stretchr/testify/require"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestValidate(t *testing.T) {
	sut := &Validator{}

	tests := []struct {
		name      string
		pipeline  telemetryv1beta1.MetricPipeline
		expectErr bool
	}{
		{
			name: "runtime input disabled",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(false).
				Build(),
		},
		{
			name: "runtime input enabled with no additional metrics",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				Build(),
		},
		{
			name: "valid kubeletstats metric",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("k8s.node.cpu.time").
				Build(),
		},
		{
			name: "valid k8scluster metric",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("k8s.deployment.available").
				Build(),
		},
		{
			name: "invalid metric",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("invalid.metric.name").
				Build(),
			expectErr: true,
		},
		{
			name: "multiple valid metrics",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("k8s.node.cpu.time", "k8s.deployment.available", "container.cpu.time").
				Build(),
		},
		{
			name: "mix of valid and invalid metrics",
			pipeline: testutils.NewMetricPipelineBuilder().
				WithRuntimeInput(true).
				WithRuntimeInputAdditionalMetrics("k8s.node.cpu.time", "invalid.metric.name").
				Build(),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sut.Validate(&tt.pipeline)
			if tt.expectErr {
				require.Error(t, err)
				require.True(t, IsInvalidAdditionalMetricError(err))
			} else {
				require.NoError(t, err)
			}
		})
	}
}
