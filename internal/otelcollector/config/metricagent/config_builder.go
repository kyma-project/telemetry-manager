package metricagent

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

type BuilderConfig struct {
	GatewayOTLPServiceName types.NamespacedName
}

type Builder struct {
	Config BuilderConfig

	config  *common.Config
	envVars common.EnvVars
}

type BuildOptions struct {
	IstioEnabled                bool
	IstioCertPath               string
	InstrumentationScopeVersion string
	AgentNamespace              string
}

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]
type componentIDFunc = common.ComponentIDFunc[*telemetryv1alpha1.MetricPipeline]
type componentConfigFunc = common.ComponentConfigFunc[*telemetryv1alpha1.MetricPipeline]
type exporterComponentConfigFunc = common.ExporterComponentConfigFunc[*telemetryv1alpha1.MetricPipeline]

var staticComponentID = common.StaticComponentID[*telemetryv1alpha1.MetricPipeline]

type fooFunc = func(pipelineID string) buildComponentFunc

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

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.config = &common.Config{
		Base:       common.BaseConfig(common.WithK8sLeaderElector("serviceAccount", common.K8sLeaderElectorK8sCluster, opts.AgentNamespace)),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
		Connectors: make(map[string]any),
	}
	b.envVars = make(common.EnvVars)

	inputs := inputSources{
		runtimeResources: runtimeResourceSources{
			pod:         shouldEnableRuntimePodMetricsScraping(pipelines),
			container:   shouldEnableRuntimeContainerMetricsScraping(pipelines),
			node:        shouldEnableRuntimeNodeMetricsScraping(pipelines),
			volume:      shouldEnableRuntimeVolumeMetricsScraping(pipelines),
			statefulset: shouldEnableRuntimeStatefulSetMetricsScraping(pipelines),
			deployment:  shouldEnableRuntimeDeploymentMetricsScraping(pipelines),
			daemonset:   shouldEnableRuntimeDaemonSetMetricsScraping(pipelines),
			job:         shouldEnableRuntimeJobMetricsScraping(pipelines),
		},

		runtime:    shouldEnableRuntimeMetricsScraping(pipelines),
		prometheus: shouldEnablePrometheusMetricsScraping(pipelines),
		istio:      shouldEnableIstioMetricsScraping(pipelines),
		envoy:      shouldEnableEnvoyMetricsScraping(pipelines),
	}

	if inputs.runtime {
		if err := b.addServicePipeline(ctx, "metrics/runtime",
			b.addK8sClusterReceiver(inputs.runtimeResources),
			b.addKubeletStatsReceiver(inputs.runtimeResources),
			b.addMemoryLimiterProcessor(),
			b.addFilterDropNonPVCVolumesMetricsProcessor(inputs.runtimeResources),
			b.addFilterDropVirtualNetworkInterfacesProcessor(),
			b.addResourceDeleteServiceNameProcessor(),
			b.addSetInstrumentationScopeToRuntimeProcessor(opts),
			b.addInsertSkipEnrichmentAttributeProcessor(),
			b.addBatchProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add runtime service pipeline: %w", err)
		}
	}

	if inputs.prometheus {
		if err := b.addServicePipeline(ctx, "metrics/prometheus",
			b.addPrometheusAppPodsReceiver(),
			b.addPrometheusAppServicesReceiver(opts),
			b.addMemoryLimiterProcessor(),
			b.addDeleteServiceNameProcessor(),
			b.addSetInstrumentationScopeToPrometheusProcessor(opts),
			b.addBatchProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add prometheus service pipeline: %w", err)
		}
	}

	if inputs.istio {
		if err := b.addServicePipeline(ctx, "metrics/istio",
			b.addPrometheusIstioReceiver(inputs.envoy),
			b.addMemoryLimiterProcessor(),
			b.addIstioNoiseFilterProcessor(),
			b.addDeleteServiceNameProcessor(),
			b.addSetInstrumentationScopeToIstioProcessor(opts),
			b.addBatchProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add istio service pipeline: %w", err)
		}
	}

	return b.config, b.envVars, nil
}

func (b *Builder) addServicePipeline(ctx context.Context, pipelineID string, fs ...fooFunc) error {
	// Initialize pipeline componentsAdd an empty pipeline to the config
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		// None of the service pipelines depend on the MetricPipeline object, so we can pass nil here
		if err := f(pipelineID)(ctx, nil); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) fooFunc {
	return func(pipelineID string) buildComponentFunc {
		return common.AddReceiver(b.config, componentIDFunc, configFunc, func(_ *telemetryv1alpha1.MetricPipeline) string {
			return pipelineID
		})
	}
}

func (b *Builder) addProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) fooFunc {
	return func(pipelineID string) buildComponentFunc {
		return common.AddProcessor(b.config, componentIDFunc, configFunc, func(_ *telemetryv1alpha1.MetricPipeline) string {
			return pipelineID
		})
	}
}

func (b *Builder) addExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) fooFunc {
	return func(pipelineID string) buildComponentFunc {
		return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, func(_ *telemetryv1alpha1.MetricPipeline) string {
			return pipelineID
		})
	}
}

// Receiver builders

func (b *Builder) addK8sClusterReceiver(runtimeResources runtimeResourceSources) fooFunc {
	return b.addReceiver(
		staticComponentID("k8s_cluster"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return k8sClusterReceiverConfig(runtimeResources)
		},
	)
}

func (b *Builder) addKubeletStatsReceiver(runtimeResources runtimeResourceSources) fooFunc {
	return b.addReceiver(
		staticComponentID("kubeletstats"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return kubeletStatsReceiverConfig(runtimeResources)
		},
	)
}

func (b *Builder) addPrometheusAppPodsReceiver() fooFunc {
	return b.addReceiver(
		staticComponentID("prometheus/app-pods"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return prometheusPodsReceiverConfig()
		},
	)
}

func (b *Builder) addPrometheusAppServicesReceiver(opts BuildOptions) fooFunc {
	return b.addReceiver(
		staticComponentID("prometheus/app-services"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return prometheusServicesReceiverConfig(opts)
		},
	)
}

func (b *Builder) addPrometheusIstioReceiver(envoyMetricsEnabled bool) fooFunc {
	return b.addReceiver(
		staticComponentID("prometheus/istio"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return prometheusIstioReceiverConfig(envoyMetricsEnabled)
		},
	)
}

// Processor builders

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("memory_limiter"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addFilterDropNonPVCVolumesMetricsProcessor(runtimeResources runtimeResourceSources) fooFunc {
	return b.addProcessor(
		staticComponentID("filter/drop-non-pvc-volumes-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !runtimeResources.volume {
				return nil
			}
			return dropNonPVCVolumesMetricsProcessorConfig()
		},
	)
}

func (b *Builder) addFilterDropVirtualNetworkInterfacesProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("filter/drop-virtual-network-interfaces"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return dropVirtualNetworkInterfacesProcessorConfig()
		},
	)
}

func (b *Builder) addResourceDeleteServiceNameProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("resource/delete-service-name"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return deleteServiceNameProcessorConfig()
		},
	)
}

func (b *Builder) addDeleteServiceNameProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("resource/delete-service-name"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return deleteServiceNameProcessorConfig()
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToRuntimeProcessor(opts BuildOptions) fooFunc {
	return b.addProcessor(
		staticComponentID("transform/set-instrumentation-scope-runtime"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceRuntime, common.InputSourceK8sCluster)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToPrometheusProcessor(opts BuildOptions) fooFunc {
	return b.addProcessor(
		staticComponentID("transform/set-instrumentation-scope-prometheus"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourcePrometheus)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToIstioProcessor(opts BuildOptions) fooFunc {
	return b.addProcessor(
		staticComponentID("transform/set-instrumentation-scope-istio"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceIstio)
		},
	)
}

func (b *Builder) addInsertSkipEnrichmentAttributeProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("transform/insert-skip-enrichment-attribute"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return insertSkipEnrichmentAttributeProcessorConfig()
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("istio_noise_filter"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() fooFunc {
	return b.addProcessor(
		staticComponentID("batch"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.BatchProcessor{
				SendBatchSize:    1024,
				Timeout:          "10s",
				SendBatchMaxSize: 1024,
			}
		},
	)
}

// Exporter builders

//nolint:mnd // all static config from here
func (b *Builder) addOTLPExporter() fooFunc {
	return b.addExporter(
		staticComponentID("otlp"),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.OTLPExporter{
				Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d", b.Config.GatewayOTLPServiceName.Name, b.Config.GatewayOTLPServiceName.Namespace, ports.OTLPGRPC),
				TLS: common.TLS{
					Insecure: true,
				},
				SendingQueue: common.SendingQueue{
					Enabled:   true,
					QueueSize: 512,
				},
				RetryOnFailure: common.RetryOnFailure{
					Enabled:         true,
					InitialInterval: "5s",
					MaxInterval:     "30s",
					MaxElapsedTime:  "300s",
				},
			}, nil, nil
		},
	)
}

// Helper functions for determining what should be enabled

func shouldEnableRuntimeMetricsScraping(pipelines []telemetryv1alpha1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) {
			return true
		}
	}

	return false
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

// Processor configuration functions (merged from processors.go)

func deleteServiceNameProcessorConfig() *common.ResourceProcessor {
	return &common.ResourceProcessor{
		Attributes: []common.AttributeAction{
			{
				Action: "delete",
				Key:    "service.name",
			},
		},
	}
}

func insertSkipEnrichmentAttributeProcessorConfig() *common.TransformProcessor {
	metricsToSkipEnrichment := []string{
		"node",
		"statefulset",
		"daemonset",
		"deployment",
		"job",
	}

	return common.MetricTransformProcessorConfig([]common.TransformProcessorStatements{{
		Conditions: metricNameConditionsWithIsMatch(metricsToSkipEnrichment),
		Statements: []string{fmt.Sprintf("set(resource.attributes[\"%s\"], \"true\")", common.SkipEnrichmentAttribute)},
	}})
}

func dropNonPVCVolumesMetricsProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Metric: []string{
				common.JoinWithAnd(
					// identify volume metrics by checking existence of "k8s.volume.name" resource attribute
					common.ResourceAttributeIsNotNil("k8s.volume.name"),
					common.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
				),
			},
		},
	}
}

func dropVirtualNetworkInterfacesProcessorConfig() *FilterProcessor {
	return &FilterProcessor{
		Metrics: FilterProcessorMetrics{
			Datapoint: []string{
				common.JoinWithAnd(
					common.IsMatch("metric.name", "^k8s.node.network.*"),
					common.Not(common.IsMatch("attributes[\"interface\"]", "^(eth|en).*")),
				),
			},
		},
	}
}

func metricNameConditionsWithIsMatch(metrics []string) []string {
	var conditions []string

	for _, m := range metrics {
		condition := common.IsMatch("metric.name", fmt.Sprintf("^k8s.%s.*", m))
		conditions = append(conditions, condition)
	}

	return conditions
}
