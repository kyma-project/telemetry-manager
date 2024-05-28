package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
	"github.com/kyma-project/telemetry-manager/internal/version"
)

var upstreamInstrumentationScopeName = map[metric.InputSourceType]string{
	metric.InputSourceRuntime:    "otelcol/kubeletstatsreceiver",
	metric.InputSourcePrometheus: "otelcol/prometheusreceiver",
	metric.InputSourceIstio:      "otelcol/prometheusreceiver",
}

func makeInstrumentationScopeProcessor(inputSource metric.InputSourceType) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Context:    "scope",
				Statements: makeInstrumentationStatement(inputSource),
			},
		},
	}
}

func makeInstrumentationStatement(inputSource metric.InputSourceType) []string {
	return []string{
		fmt.Sprintf("set(version, \"%s\") where name == \"%s\"", version.Version, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(name, \"%s\") where name == \"\" or name == \"%s\"", metric.InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}
