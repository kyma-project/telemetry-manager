package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

var upstreamInstrumentationScopeName = map[metric.InputSourceType]string{
	metric.InputSourceRuntime:    "otelcol/kubeletstatsreceiver",
	metric.InputSourcePrometheus: "otelcol/prometheusreceiver",
	metric.InputSourceIstio:      "otelcol/prometheusreceiver",
}

func makeInstrumentationScopeProcessor(inputSource metric.InputSourceType, opts BuildOptions) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Context:    "scope",
				Statements: makeInstrumentationStatement(inputSource, opts),
			},
		},
	}
}

func makeInstrumentationStatement(inputSource metric.InputSourceType, opts BuildOptions) []string {
	return []string{
		fmt.Sprintf("set(version, \"%s\") where name == \"%s\"", opts.InstrumentationScopeVersion, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(name, \"%s\") where name == \"%s\"", metric.InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}
