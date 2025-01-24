package agent

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"k8s.io/apimachinery/pkg/types"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}
type Builder struct {
	Config BuilderConfig
}

type BuildOptions struct {
	InstrumentationScopeVersion string
	AgentNamespace              string
}

func (b *Builder) Build(opts BuildOptions) *Config {
	logService := config.DefaultService(makePipelinesConfig())
	// Overwrite the extension from default service name
	logService.Extensions = []string{"health_check", "pprof", "file_storage"}
	return &Config{
		Base: config.Base{
			Service: logService,
		},

		Receivers:  makeReceivers(),
		Processors: makeProcessorsConfig(opts.InstrumentationScopeVersion),
		Exporters:  makeExportersConfig(b.Config.GatewayOTLPServiceName),
		Extensions: makeExtensionsConfig(),
	}
}

func makePipelinesConfig() config.Pipelines {
	pipelinesConfig := make(config.Pipelines)
	pipelinesConfig["logs"] = config.Pipeline{
		Receivers:  []string{"filelog"},
		Processors: []string{"memory_limiter", "transform/set-instrumentation-scope-runtime"},
		Exporters:  []string{"otlp"},
	}
	return pipelinesConfig
}

func makeExportersConfig(gatewayServiceName types.NamespacedName) Exporters {
	return Exporters{
		OTLP: &config.OTLPExporter{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, ports.OTLPGRPC),
			TLS: config.TLS{
				Insecure: true,
			},
			SendingQueue: config.SendingQueue{
				Enabled: false,
			},
			RetryOnFailure: config.RetryOnFailure{
				Enabled:         true,
				InitialInterval: "5s",
				MaxInterval:     "30s",
				MaxElapsedTime:  "300s",
			},
		},
	}
}
