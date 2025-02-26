package metric

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

var upstreamInstrumentationScopeName = map[InputSourceType]string{
	InputSourceRuntime:    "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver",
	InputSourcePrometheus: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceIstio:      "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver",
	InputSourceKyma:       "github.com/kyma-project/opentelemetry-collector-components/receiver/kymastatsreceiver",
	InputSourceK8sCluster: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver",
}

func MakeInstrumentationScopeProcessor(instrumentationScopeVersion string, inputSource ...InputSourceType) *TransformProcessor {
	statements := []string{}
	for _, i := range inputSource {
		statements = append(statements, makeInstrumentationStatement(i, instrumentationScopeVersion)...)
		if i == InputSourcePrometheus {
			statements = append(statements, []string{fmt.Sprintf("set(resource.attributes[\"%s\"], \"%s\")", "kyma.input.name", "prometheus")}...)
		}
	}

	return &TransformProcessor{
		ErrorMode: "ignore",
		MetricStatements: []config.TransformProcessorStatements{
			{
				Statements: statements,
			},
		},
	}
}

func makeInstrumentationStatement(inputSource InputSourceType, instrumentationScopeVersion string) []string {
	return []string{
		fmt.Sprintf("set(scope.version, \"%s\") where scope.name == \"%s\"", instrumentationScopeVersion, upstreamInstrumentationScopeName[inputSource]),
		fmt.Sprintf("set(scope.name, \"%s\") where scope.name == \"%s\"", InstrumentationScope[inputSource], upstreamInstrumentationScopeName[inputSource]),
	}
}
