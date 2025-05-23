package prober

import (
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober/mocks"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestOTelLogAgentProber(t *testing.T) {
	testCases := []struct {
		name         string
		alerts       promv1.AlertsResult
		alertsErr    error
		pipelineName string
		expected     OTelAgentProbeResult
		expectErr    bool
	}{
		{
			name:         "alert getter fails",
			pipelineName: "cls",
			alertsErr:    assert.AnError,
			expectErr:    true,
		},
		{
			name:         "no alerts firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "unknown alert firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "UnknownAlert",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "pending alert should be ignored",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "LogAgentAllDataDropped",
						},
						State: promv1.AlertStatePending,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "alert missing pipeline_name label should be mapped to any pipeline",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "LogAgentAllDataDropped",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
					Healthy:        false,
				},
			},
		},
		{
			name:         "pipeline_name label does not match pipeline name",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentAllDataDropped",
							"pipeline_name": "pipeline-1",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "overlapping pipeline names",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentAllDataDropped",
							"pipeline_name": "cls-2",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "flow type mismatch",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "MetricGatewayAllDataDropped",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname":     "TraceGatewayAllDataDropped",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "all data dropped alert firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentAllDataDropped",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
				},
			},
		},
		{
			name:         "some data dropped alert firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentSomeDataDropped",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					SomeDataDropped: true,
				},
			},
		},
		{
			name:         "queue almost full alert firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentQueueAlmostFull",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelAgentProbeResult{
				QueueAlmostFull: true,
			},
		},
		{
			name:         "healthy",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{},
				},
			},
			expected: OTelAgentProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sut, err := NewOTelLogAgentProber(types.NamespacedName{Name: "test"})
			require.NoError(t, err)

			alertGetterMock := &mocks.AlertGetter{}
			if tc.alertsErr != nil {
				alertGetterMock.On("Alerts", mock.Anything).Return(promv1.AlertsResult{}, tc.alertsErr)
			} else {
				alertGetterMock.On("Alerts", mock.Anything).Return(tc.alerts, nil)
			}

			sut.getter = alertGetterMock

			result, err := sut.Probe(t.Context(), tc.pipelineName)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
