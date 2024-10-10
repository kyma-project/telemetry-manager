package metric

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/ottlexpr"
)

var upstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver",
	InputSourcePrometheus: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceIstio:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceKyma:       "github.com/kyma-project/opentelemetry-collector-components/receiver/kymastatsreceiver",
	InputSourceK8sCluster: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver",
}

func MakeInstrumentationScopeProcessor(inputSource InputSourceType, instrumentationScopeVersion string) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Context:    "scope",
				Statements: makeInstrumentationStatement(inputSource, instrumentationScopeVersion),
			},
		},
	}
}

// MakeInstrumentScopeRuntime sets the instrumentation scope to runtime for 2 conditions, and we want to do it in same processor
func MakeInstrumentScopeRuntime(instrumentationScopeVersion string, inputSource ...InputSourceType) *TransformProcessor {
	conditions := []string{}
	for _, i := range inputSource {
		conditions = append(conditions, ottlexpr.IsMatch("name", upstreamInstrumentationScopeName[i]))
	}
	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Context: "scope",
				Statements: []string{
					fmt.Sprintf("set(version, \"%s\")", instrumentationScopeVersion),
					fmt.Sprintf("set(name, \"%s\")", InstrumentationScope[InputSourceRuntime]),
				},
				Conditions: conditions,
			},
		},
	}
}

func makeInstrumentationStatement(inputSource InputSourceType, instrumentationScopeVersion string) []string {
	return []string{
		fmt.Sprintf("set(version, \"%s\") where name == \"%s\"", instrumentationScopeVersion, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(name, \"%s\") where name == \"%s\"", InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}
