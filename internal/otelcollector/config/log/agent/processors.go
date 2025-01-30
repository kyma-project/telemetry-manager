package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
)

const (
	limitPercentage      = 80
	spikeLimitPercentage = 25
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
		LimitPercentage:      limitPercentage,
		SpikeLimitPercentage: spikeLimitPercentage,
	}
}

func makeInstrumentationScopeRuntime(instrumentationScopeVersion string) *log.TransformProcessor {
	return &log.TransformProcessor{
		ErrorMode: "ignore",
		LogStatements: []config.TransformProcessorStatements{
			{
				Context: "scope",
				Statements: []string{
					fmt.Sprintf("set(version, \"%s\")", instrumentationScopeVersion),
					fmt.Sprintf("set(name, \"%s\")", "io.kyma-project.telemetry/runtime"),
				},
			},
		},
	}
}
