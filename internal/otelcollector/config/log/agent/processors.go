package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/log"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"
)

const (
	limitPercentage             = 80
	spikeLimitPercentage        = 25
	InstrumentationScopeRuntime = "io.kyma-project.telemetry/runtime"
)

func makeProcessorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			MemoryLimiter: makeMemoryLimiterConfig(),
		},
		SetInstrumentationScopeRuntime: makeInstrumentationScopeRuntime(opts.InstrumentationScopeVersion),
		K8sAttributes:                  processors.K8sAttributesProcessorConfig(opts.Enrichments),
		InsertClusterAttributes:        processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.CloudProvider),
		DropKymaAttributes:             processors.DropKymaAttributesProcessorConfig(),
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
					fmt.Sprintf("set(version, %q)", instrumentationScopeVersion),
					fmt.Sprintf("set(name, %q)", InstrumentationScopeRuntime),
				},
			},
		},
	}
}
