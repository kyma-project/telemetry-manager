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

func TestOTelPipelineProber(t *testing.T) {
	testCases := []struct {
		name         string
		alerts       promv1.AlertsResult
		alertsErr    error
		pipelineName string
		expected     OTelPipelineProbeResult
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
			expected: OTelPipelineProbeResult{
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
							"alertname": "UnknownAlert",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
		{
			name:         "alert missing exporter label",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterDroppedData",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
				},
			},
		},
		{
			name:         "exporter label does not match pipeline name",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterDroppedData",
							"exporter":  "otlp/dynatrace",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
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
							"alertname": "TraceGatewayExporterDroppedData",
							"exporter":  "otlp/cls-2",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
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
							"alertname": "MetricGatewayExporterDroppedData",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname": "LogAgentBufferFull",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
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
							"alertname": "TraceGatewayExporterDroppedData",
							"exporter":  "otlphttp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					AllDataDropped: true,
				},
			},
		},
		{
			name:         "exporter sent data and exporter dropped data firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterDroppedData",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterSentData",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					SomeDataDropped: true,
				},
			},
		},
		{
			name:         "exporter sent data and exporter enqueue failed firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterEnqueueFailed",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterSentData",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					SomeDataDropped: true,
				},
			},
		},
		{
			name:         "exporter sent data and exporter dropped data and exporter enqueue failed firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterEnqueueFailed",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterDroppedData",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterSentData",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					SomeDataDropped: true,
				},
			},
		},
		{
			name:         "queue almost full firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterQueueAlmostFull",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{QueueAlmostFull: true},
		},
		{
			name:         "receiver refused firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayReceiverRefusedData",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: OTelPipelineProbeResult{Throttling: true},
		},
		{
			name:         "healthy",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "TraceGatewayExporterSentData",
							"exporter":  "otlp/cls",
						},
					},
				},
			},
			expected: OTelPipelineProbeResult{
				PipelineProbeResult: PipelineProbeResult{
					Healthy: true,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sut, err := NewTracePipelineProber(types.NamespacedName{Name: "test"})
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
