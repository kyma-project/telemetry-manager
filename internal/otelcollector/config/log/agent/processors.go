package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
)

const (
	limitPercentage             = 80
	spikeLimitPercentage        = 25
	InstrumentationScopeRuntime = "io.kyma-project.telemetry/runtime"
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
				Statements: []string{
					fmt.Sprintf("set(scope.version, %q)", instrumentationScopeVersion),
					fmt.Sprintf("set(scope.name, %q)", InstrumentationScopeRuntime),
				},
			},
		},
	}
}
