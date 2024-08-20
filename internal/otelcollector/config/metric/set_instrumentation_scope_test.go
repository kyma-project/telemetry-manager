package metric

import (
	"testing"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func TestTransformedInstrumentationScope(t *testing.T) {
	instrumentationScopeVersion := "main"
	tests := []struct {
		name        string
		want        *TransformProcessor
		inputSource InputSourceType
	}{
		{
			name: "InputSourceRuntime",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context: "scope",
					Statements: []string{
						"set(version, \"main\") where name == \"otelcol/kubeletstatsreceiver\"",
						"set(name, \"io.kyma-project.telemetry/runtime\") where name == \"otelcol/kubeletstatsreceiver\"",
					},
				}},
			},
			inputSource: InputSourceRuntime,
		}, {
			name: "InputSourcePrometheus",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context: "scope",
					Statements: []string{
						"set(version, \"main\") where name == \"otelcol/prometheusreceiver\"",
						"set(name, \"io.kyma-project.telemetry/prometheus\") where name == \"otelcol/prometheusreceiver\"",
					},
				}},
			},
			inputSource: InputSourcePrometheus,
		}, {
			name: "InputSourceIstio",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context: "scope",
					Statements: []string{
						"set(version, \"main\") where name == \"otelcol/prometheusreceiver\"",
						"set(name, \"io.kyma-project.telemetry/istio\") where name == \"otelcol/prometheusreceiver\"",
					},
				}},
			},
			inputSource: InputSourceIstio,
		}, {
			name: "InputSourceK8sCluster",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context: "scope",
					Statements: []string{
						"set(version, \"main\") where name == \"otelcol/kymastats\"",
						"set(name, \"io.kyma-project.telemetry/kyma\") where name == \"otelcol/kymastats\"",
					},
				}},
			},
			inputSource: InputSourceKyma,
		}, {
			name: "InputSourceK8sCluster",
			want: &TransformProcessor{
				ErrorMode: "ignore",
				MetricStatements: []config.TransformProcessorStatements{{
					Context: "scope",
					Statements: []string{
						"set(version, \"main\") where name == \"otelcol/k8sclusterreceiver\"",
						"set(name, \"io.kyma-project.telemetry/runtime\") where name == \"otelcol/k8sclusterreceiver\"",
					},
				}},
			},
			inputSource: InputSourceK8sCluster,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MakeInstrumentationScopeProcessor(tt.inputSource, instrumentationScopeVersion); !compareTransformProcessor(got, tt.want) {
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
