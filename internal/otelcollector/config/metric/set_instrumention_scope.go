package metric

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

var upstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver",
	InputSourcePrometheus: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceIstio:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceKyma:       "otelcol/kymastats",
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

func makeInstrumentationStatement(inputSource InputSourceType, instrumentationScopeVersion string) []string {
	return []string{
		fmt.Sprintf("set(version, \"%s\") where name == \"%s\"", instrumentationScopeVersion, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(name, \"%s\") where name == \"%s\"", InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}
