package agent

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/processors"

	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
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
	ClusterName                 string
	CloudProvider               string
	Enrichments                 processors.Enrichments
}

func (b *Builder) Build(logPipelines []telemetryv1alpha1.LogPipeline, opts BuildOptions) *Config {
	logService := config.DefaultService(makePipelinesConfig())
	// Overwrite the extension from default service name
	logService.Extensions = []string{"health_check", "pprof", "file_storage"}

	return &Config{
		Service:    logService,
		Extensions: makeExtensionsConfig(),

		// have filelog receiver for each log pipeline
		Receivers: makeReceivers(logPipelines, opts),
		// Add k8s attributes and resource/insert clustername
		Processors: makeProcessorsConfig(opts),
		// Push to endpoint directly
		Exporters: makeExportersConfig(b.Config.GatewayOTLPServiceName),
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
