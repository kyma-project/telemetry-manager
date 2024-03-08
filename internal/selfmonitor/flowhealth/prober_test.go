package flowhealth

import (
	"context"
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/flowhealth/mocks"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProber(t *testing.T) {
	testCases := []struct {
		name         string
		alerts       promv1.AlertsResult
		alertsErr    error
		pipelineName string
		expected     ProbeResult
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
			expected: ProbeResult{},
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
			expected: ProbeResult{},
		},
		{
			name:         "GatewayExporterDroppedMetricPoints firing",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "GatewayExporterDroppedMetricPoints",
							"exporter":  "otlp/cls",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: ProbeResult{AllDataDropped: true},
		},
		{
			name:         "GatewayExporterDroppedMetricPoints firing with missing exporter label",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "GatewayExporterDroppedMetricPoints",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: ProbeResult{},
		},
		{
			name:         "GatewayExporterDroppedMetricPoints firing with different exporter",
			pipelineName: "cls",
			alerts: promv1.AlertsResult{
				Alerts: []promv1.Alert{
					{
						Labels: model.LabelSet{
							"alertname": "GatewayExporterDroppedMetricPoints",
							"exporter":  "otlp/dynatrace",
						},
						State: promv1.AlertStateFiring,
					},
				},
			},
			expected: ProbeResult{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alertGetterMock := &mocks.AlertGetter{}

			if tc.alertsErr != nil {
				alertGetterMock.On("Alerts", mock.Anything).Return(promv1.AlertsResult{}, tc.alertsErr)
			} else {
				alertGetterMock.On("Alerts", mock.Anything).Return(tc.alerts, nil)
			}

			sut := Prober{
				getter: alertGetterMock,
			}

			ctx := context.TODO()
			result, err := sut.Probe(ctx, tc.pipelineName)

			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result)
			}
		})
	}
}
