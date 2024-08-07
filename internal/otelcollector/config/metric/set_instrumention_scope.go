package metric

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

var upstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "otelcol/kubeletstatsreceiver",
	InputSourcePrometheus: "otelcol/prometheusreceiver",
	InputSourceIstio:      "otelcol/prometheusreceiver",
	InputSourceKyma:       "otelcol/kymastats",
	InputSourceK8sCluster: "otelcol/k8sclusterreceiver",
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
