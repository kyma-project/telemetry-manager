package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"testing"
)

func Test_transformedInstrumentationScope(t *testing.T) {
	tests := []struct {
		name        string
		want        *TransformProcessor
		inputSource metric.InputSourceType
	}{
		{
			name: "InputSourceRuntime",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context:    "scope",
					Statements: []string{"set name, io.kyma-project.telemetry/kubeletstatsreceiver) where name == \"\" or name == otelcol/kubeletstatsreceiver"},
				}},
			},
			inputSource: metric.InputSourceRuntime,
		}, {
			name: "InputSourcePrometheus",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context:    "scope",
					Statements: []string{"set name, io.kyma-project.telemetry/prometheusreceiver) where name == \"\" or name == otelcol/prometheusreceiver"},
				}},
			},
			inputSource: metric.InputSourcePrometheus,
		}, {
			name: "InputSourceIstio",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context:    "scope",
					Statements: []string{"set name, io.kyma-project.telemetry/prometheusreceiver) where name == \"\" or name == otelcol/prometheusreceiver"},
				}},
			},
			inputSource: metric.InputSourceIstio,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transformInstrumentationScope(tt.inputSource); !compareTransformProcessor(got, tt.want) {
				t.Errorf("transformInstrumentationScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func compareTransformProcessor(got, want *TransformProcessor) bool {
	if got.ErrorMode != want.ErrorMode {
		return false
	}
	if len(got.MetricStatements) != len(want.MetricStatements) {
		return false
	}
	for i, statement := range got.MetricStatements {
		if statement.Context != want.MetricStatements[i].Context {
			return false
		}
		if len(statement.Statements) != len(want.MetricStatements[i].Statements) {
			return false
		}
		for j, s := range statement.Statements {
			if s != want.MetricStatements[i].Statements[j] {
				return false
			}
		}
	}
	return true
}
