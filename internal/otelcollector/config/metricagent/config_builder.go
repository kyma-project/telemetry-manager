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

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.MetricPipeline]

	GatewayOTLPServiceName types.NamespacedName
}

type BuildOptions struct {
	IstioEnabled                bool
	IstioCertPath               string
	InstrumentationScopeVersion string
	AgentNamespace              string
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

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, error) {
	b.Config = common.NewConfig()
	b.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElector{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorK8sCluster,
			LeaseNamespace: opts.AgentNamespace,
		},
	)

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
		if err := b.AddServicePipeline(ctx, nil, "metrics/runtime",
			b.addKubeletStatsReceiver(inputs.runtimeResources),
			b.addK8sClusterReceiver(inputs.runtimeResources),
			b.addMemoryLimiterProcessor(),
			b.addFilterDropNonPVCVolumesMetricsProcessor(inputs.runtimeResources),
			b.addFilterDropVirtualNetworkInterfacesProcessor(),
			b.addResourceDeleteServiceNameProcessor(),
			b.addSetInstrumentationScopeToRuntimeProcessor(opts),
			b.addInsertSkipEnrichmentAttributeProcessor(),
			b.addBatchProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, fmt.Errorf("failed to add runtime service pipeline: %w", err)
		}
	}

	if inputs.prometheus {
		if err := b.AddServicePipeline(ctx, nil, "metrics/prometheus",
			b.addPrometheusAppPodsReceiver(),
			b.addPrometheusAppServicesReceiver(opts),
			b.addMemoryLimiterProcessor(),
			b.addDeleteServiceNameProcessor(),
			b.addSetInstrumentationScopeToPrometheusProcessor(opts),
			b.addBatchProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, fmt.Errorf("failed to add prometheus service pipeline: %w", err)
		}
	}

	if inputs.istio {
		if err := b.AddServicePipeline(ctx, nil, "metrics/istio",
			b.addPrometheusIstioReceiver(inputs.envoy),
			b.addMemoryLimiterProcessor(),
			b.addIstioNoiseFilterProcessor(),
			b.addDeleteServiceNameProcessor(),
			b.addSetInstrumentationScopeToIstioProcessor(opts),
			b.addBatchProcessor(),
			b.addOTLPExporter(),
		); err != nil {
			return nil, fmt.Errorf("failed to add istio service pipeline: %w", err)
		}
	}

	return b.Config, nil
}

// Receiver builders

func (b *Builder) addK8sClusterReceiver(runtimeResources runtimeResourceSources) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDK8sClusterReceiver),
		func(*telemetryv1alpha1.MetricPipeline) any {
			return k8sClusterReceiverConfig(runtimeResources)
		},
	)
}

func (b *Builder) addKubeletStatsReceiver(runtimeResources runtimeResourceSources) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDKubeletStatsReceiver),
		func(*telemetryv1alpha1.MetricPipeline) any {
			return kubeletStatsReceiverConfig(runtimeResources)
		},
	)
}

func (b *Builder) addPrometheusAppPodsReceiver() buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDPrometheusAppPodsReceiver),
		func(*telemetryv1alpha1.MetricPipeline) any {
			return prometheusPodsReceiverConfig()
		},
	)
}

func (b *Builder) addPrometheusAppServicesReceiver(opts BuildOptions) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDPrometheusAppServicesReceiver),
		func(*telemetryv1alpha1.MetricPipeline) any {
			return prometheusServicesReceiverConfig(opts)
		},
	)
}

func (b *Builder) addPrometheusIstioReceiver(envoyMetricsEnabled bool) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDPrometheusIstioReceiver),
		func(*telemetryv1alpha1.MetricPipeline) any {
			return prometheusIstioReceiverConfig(envoyMetricsEnabled)
		},
	)
}

// Processor builders

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addFilterDropNonPVCVolumesMetricsProcessor(runtimeResources runtimeResourceSources) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDFilterDropNonPVCVolumesMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !runtimeResources.volume {
				return nil
			}

			return dropNonPVCVolumesMetricsProcessorConfig()
		},
	)
}

func (b *Builder) addFilterDropVirtualNetworkInterfacesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDFilterDropVirtualNetworkInterfacesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return dropVirtualNetworkInterfacesProcessorConfig()
		},
	)
}

func (b *Builder) addResourceDeleteServiceNameProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDResourceDeleteServiceNameProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return deleteServiceNameProcessorConfig()
		},
	)
}

func (b *Builder) addDeleteServiceNameProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDResourceDeleteServiceNameProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return deleteServiceNameProcessorConfig()
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToRuntimeProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeRuntimeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceRuntime, common.InputSourceK8sCluster)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToPrometheusProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopePrometheusProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourcePrometheus)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToIstioProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeIstioProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceIstio)
		},
	)
}

func (b *Builder) addInsertSkipEnrichmentAttributeProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertSkipEnrichmentAttributeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return insertSkipEnrichmentAttributeProcessorConfig()
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.IstioNoiseFilterProcessor{}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDBatchProcessor),
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
func (b *Builder) addOTLPExporter() buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(common.ComponentIDOTLPExporter),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.OTLPExporter{
				Endpoint: fmt.Sprintf("%s.%s.svc.cluster.local:%d",
					b.GatewayOTLPServiceName.Name,
					b.GatewayOTLPServiceName.Namespace,
					ports.OTLPGRPC,
				),
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
