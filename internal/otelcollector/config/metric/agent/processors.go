package agent

import (
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

func makeProcessorsConfig(inputs inputSources) Processors {
	var processorsConfig Processors

	if inputs.runtime || inputs.prometheus || inputs.istio {
		processorsConfig.DeleteServiceName = makeDeleteServiceNameConfig()

		if inputs.runtime {
			processorsConfig.InsertInputSourceRuntime = makeEmittedByConfig(metric.InputSourceRuntime)
		}

		if inputs.prometheus {
			processorsConfig.InsertInputSourcePrometheus = makeEmittedByConfig(metric.InputSourcePrometheus)
		}

		if inputs.istio {
			processorsConfig.InsertInputSourceIstio = makeEmittedByConfig(metric.InputSourceIstio)
		}
	}

	return processorsConfig
}

func makeDeleteServiceNameConfig() *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func makeEmittedByConfig(inputSource metric.InputSourceType) *config.ResourceProcessor {
	return &config.ResourceProcessor{
		Attributes: []config.AttributeAction{
			{
				Action: "insert",
				Key:    metric.InputSourceAttribute,
				Value:  string(inputSource),
			},
		},
	}
}
