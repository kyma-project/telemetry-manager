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
	var instrumentationScopeName = map[metric.InputSourceType]string{
		metric.InputSourceRuntime:    "otelcol/kubeletstatsreceiver",
		metric.InputSourcePrometheus: "otelcol/prometheusreceiver",
		metric.InputSourceIstio:      "otelcol/prometheusreceiver",
	}

	var transformedInstrumentationScopeName = map[metric.InputSourceType]string{
		metric.InputSourceRuntime:    "io.kyma-project.telemetry/kubeletstatsreceiver",
		metric.InputSourcePrometheus: "io.kyma-project.telemetry/prometheusreceiver",
		metric.InputSourceIstio:      "io.kyma-project.telemetry/prometheusreceiver",
	}

	return fmt.Sprintf("set name, %s) where name == \"\" or name == %s", transformedInstrumentationScopeName[inputSource], instrumentationScopeName[inputSource])
}
