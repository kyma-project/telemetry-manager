package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func transformInstrumentationScope(inputSource metric.InputSourceType) *TransformProcessor {
	return &TransformProcessor{
		ErrorMode:        "ignore",
		MetricStatements: makeMetricStatement(inputSource),
	}
}

func makeMetricStatement(inputSource metric.InputSourceType) []config.TransformProcessorStatements {
	var metricStatements []string

	return []config.TransformProcessorStatements{
		{
			Context:    "scope",
			Statements: append(metricStatements, makeInstrumentationStatement(inputSource)),
		},
	}
}

func makeInstrumentationStatement(inputSource metric.InputSourceType) string {

	return fmt.Sprintf("set(name, \"%s\") where name == \"\" or name == \"%s\"", metric.TransformedInstrumentationScope[inputSource], metric.UpstreamInstrumentationScopeName[inputSource])
}
