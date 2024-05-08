package agent

import (
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func TestTransformedInstrumentationScope(t *testing.T) {
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
					Statements: []string{"set(name, \"io.kyma-project.telemetry/runtime\") where name == \"\" or name == \"otelcol/kubeletstatsreceiver\""},
				}},
			},
			inputSource: metric.InputSourceRuntime,
		}, {
			name: "InputSourcePrometheus",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context:    "scope",
					Statements: []string{"set(name, \"io.kyma-project.telemetry/prometheus\") where name == \"\" or name == \"otelcol/prometheusreceiver\""},
				}},
			},
			inputSource: metric.InputSourcePrometheus,
		}, {
			name: "InputSourceIstio",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context:    "scope",
					Statements: []string{"set(name, \"io.kyma-project.telemetry/istio\") where name == \"\" or name == \"otelcol/prometheusreceiver\""},
				}},
			},
			inputSource: metric.InputSourceIstio,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeInstrumentationScopeProcessor(tt.inputSource); !compareTransformProcessor(got, tt.want) {
				t.Errorf("makeInstrumentationScopeProcessor() = %v, want %v", got, tt.want)
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
