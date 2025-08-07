package agent

import (
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}

type Builder struct {
	Config BuilderConfig
}

// inputSources represents the enabled input sources for the telemetry metric agent.
type inputSources struct {
	runtime          bool
	runtimeResources runtimeResourceSources
	prometheus       bool
	istio            bool
	envoy            bool
}

// runtimeResourceSources represents the resources for which runtime metrics scraping is enabled.
type runtimeResourceSources struct {
	pod         bool
	container   bool
	node        bool
	volume      bool
	statefulset bool
	deployment  bool
	daemonset   bool
	job         bool
}

type BuildOptions struct {
	IstioEnabled                bool
	IstioCertPath               string
	InstrumentationScopeVersion string
	AgentNamespace              string
}

func (b *Builder) Build(pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) *Config {
	inputs := inputSources{
		runtime:          shouldEnableRuntimeMetricsScraping(pipelines),
		runtimeResources: shouldEnableRuntimeResourcesMetricsScraping(pipelines),
		prometheus:       shouldEnablePrometheusMetricsScraping(pipelines),
		istio:            shouldEnableIstioMetricsScraping(pipelines),
		envoy:            shouldEnableEnvoyMetricsScraping(pipelines),
	}

	return b.baseConfig(inputs, b.Config.GatewayOTLPServiceName, opts)
}

func shouldEnableRuntimeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeResourcesMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) runtimeResourceSources {
	return runtimeResourceSources{
		pod:         shouldEnableRuntimePodMetricsScraping(pipelines),
		container:   shouldEnableRuntimeContainerMetricsScraping(pipelines),
		node:        shouldEnableRuntimeNodeMetricsScraping(pipelines),
		volume:      shouldEnableRuntimeVolumeMetricsScraping(pipelines),
		statefulset: shouldEnableRuntimeStatefulSetMetricsScraping(pipelines),
		deployment:  shouldEnableRuntimeDeploymentMetricsScraping(pipelines),
		daemonset:   shouldEnableRuntimeDaemonSetMetricsScraping(pipelines),
		job:         shouldEnableRuntimeJobMetricsScraping(pipelines),
	}
}

func shouldEnableRuntimePodMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimePodInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeContainerMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeNodeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeVolumeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeStatefulSetMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeDeploymentMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeDaemonSetMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeJobMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeJobInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnablePrometheusMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsPrometheusInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableIstioMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsIstioInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableEnvoyMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsIstioInputEnabled(input) && metricpipelineutils.IsEnvoyMetricsEnabled(input) {
			return true
		}
	}

	return false
}

// baseConfig creates the static/global base configuration for the metric agent collector.
// Pipeline-specific components are added later via the Build method.
func (b *Builder) baseConfig(inputs inputSources, gatewayOTLPServiceName types.NamespacedName, opts BuildOptions) *Config {
	return &Config{
		Base: config.DefaultBaseConfig(
			pipelinesConfig(inputs),
			config.WithK8sLeaderElector("serviceAccount", "telemetry-metric-agent-k8scluster", opts.AgentNamespace),
		),
		Receivers:  receiversConfig(inputs, opts),
		Processors: processorsConfig(inputs, opts.InstrumentationScopeVersion),
		Exporters:  exportersConfig(gatewayOTLPServiceName),
	}
}

//nolint:mnd // all static config from here
func exportersConfig(gatewayServiceName types.NamespacedName) Exporters {
	return Exporters{
		OTLP: config.OTLPExporter{
			Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", gatewayServiceName.Name, gatewayServiceName.Namespace, ports.OTLPGRPC),
			TLS: config.TLS{
				Insecure: true,
			},
			SendingQueue: config.SendingQueue{
				Enabled:   true,
				QueueSize: 512,
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
