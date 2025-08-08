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

func processorsConfig(opts BuildOptions) Processors {
	return Processors{
		BaseProcessors: config.BaseProcessors{
			MemoryLimiter: memoryLimiterProcessorConfig(),
		},
		SetInstrumentationScopeRuntime: instrumentationScopeRuntimeProcessorConfig(opts.InstrumentationScopeVersion),
		K8sAttributes:                  processors.K8sAttributesProcessorConfig(opts.Enrichments),
		InsertClusterAttributes:        processors.InsertClusterAttributesProcessorConfig(opts.ClusterName, opts.ClusterUID, opts.CloudProvider),
		ResolveServiceName:             processors.MakeResolveServiceNameConfig(),
		DropKymaAttributes:             processors.DropKymaAttributesProcessorConfig(),
	}
}

func memoryLimiterProcessorConfig() *config.MemoryLimiter {
	return &config.MemoryLimiter{
		CheckInterval:        "5s",
		LimitPercentage:      limitPercentage,
		SpikeLimitPercentage: spikeLimitPercentage,
	}
}

func instrumentationScopeRuntimeProcessorConfig(instrumentationScopeVersion string) *log.TransformProcessor {
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
