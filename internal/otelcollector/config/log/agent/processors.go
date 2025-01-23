package agent

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

func makeProcessorsConfig(instrumentationScopeVersion string) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		SetInstrumentationScopeRuntime: makeInstrumentationScopeRuntime(instrumentationScopeVersion),
	}
}

func makeMemoryLimiterConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "5s",
		LimitPercentage:      80,
		SpikeLimitPercentage: 25,
	}
}

func makeInstrumentationScopeRuntime(instrumentationScopeVersion string) *config.TransformProcessorStatements {
	return &config.TransformProcessorStatements{
		Context: "scope",
		Statements: []string{
			fmt.Sprintf("set(version, \"%s\")", instrumentationScopeVersion),
			fmt.Sprintf("set(name, \"%s\")", "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver"),
		},
	}
}
