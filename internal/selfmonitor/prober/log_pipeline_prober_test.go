package prober

import (
	"context"
	"testing"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober/mocks"
)

func TestLogPipelineProber(t *testing.T) {
	testCases := []struct {
		name         string
		alerts       promv1.AlertsResult
		alertsErr    error
		pipelineName string
		expected     LogPipelineProbeResult
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
			expected: LogPipelineProbeResult{
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
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "alert missing pipeline_name label",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "LogAgentBufferFull",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
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
							"alertname":     "LogAgentBufferFull",
							"pipeline_name": "dynatrace",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
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
							"alertname":     "MetricGatewayExporterDroppedData",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname":     "TraceGatewayExporterDroppedData",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "exporter dropped data firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentExporterDroppedLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
				},
			},
		},
		{
			name:         "buffer full firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentBufferFull",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
				},
			},
		},
		{
			name:         "exporter sent logs and exporter dropped logs firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentExporterDroppedLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentExporterSentLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					SomeDataDropped: true,
				},
			},
		},
		{
			name:         "exporter sent logs and buffer full firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentBufferFull",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentExporterSentLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					SomeDataDropped: true,
				},
			},
		},
		{
			name:         "receiver read logs and exporter did not send logs firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentReceiverReadLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				NoLogsDelivered: true,
			},
		},
		{
			name:         "exporter read logs and exporter sent logs firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentReceiverReadLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname":     "LogAgentExporterSentLogs",
							"pipeline_name": "cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: LogPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sut, err := NewLogPipelineProber(types.NamespacedName{})
			require.NoError(t, err)

			alertGetterMock := &mocks.AlertGetter{}
			if tc.alertsErr != nil {
				alertGetterMock.On("Alerts", mock.Anything).Return(promv1.AlertsResult{}, tc.alertsErr)
			} else {
				alertGetterMock.On("Alerts", mock.Anything).Return(tc.alerts, nil)
			}
			sut.getter = alertGetterMock

			result, err := sut.Probe(context.Background(), tc.pipelineName)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
