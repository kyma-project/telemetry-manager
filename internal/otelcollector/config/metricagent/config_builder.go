package metricagent

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

const enrichmentServicePipelineID = "metrics/enrichment-conditional"

var diagnosticMetricNames = []string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"}

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1alpha1.MetricPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	IstioEnabled                bool
	IstioCertPath               string
	InstrumentationScopeVersion string
	AgentNamespace              string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
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

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	// Sort pipelines to ensure consistent order and checksum for generated ConfigMap
	slices.SortFunc(pipelines, func(a, b telemetryv1alpha1.MetricPipeline) int {
		return strings.Compare(a.Name, b.Name)
	})

	b.Config = common.NewConfig()
	b.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElector{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorK8sCluster,
			LeaseNamespace: opts.AgentNamespace,
		},
	)
	b.EnvVars = make(common.EnvVars)

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

	// Input pipelines
	pipelinesWithRuntimeInput := getPipelinesWithRuntimeInput(pipelines)
	pipelinesWithPrometheusInput := getPipelinesWithPrometheusInput(pipelines)
	pipelinesWithIstioInput := getPipelinesWithIstioInput(pipelines)

	if inputs.runtime {
		if err := b.AddServicePipeline(ctx, nil, "metrics/input-runtime",
			b.addKubeletStatsReceiver(inputs.runtimeResources),
			b.addK8sClusterReceiver(inputs.runtimeResources),
			b.addMemoryLimiterProcessor(),
			b.addFilterDropNonPVCVolumesMetricsProcessor(inputs.runtimeResources),
			b.addFilterDropVirtualNetworkInterfacesProcessor(),
			b.addDropServiceNameProcessor(),
			b.addInsertSkipEnrichmentAttributeProcessor(),
			b.addSetInstrumentationScopeToRuntimeProcessor(opts),
			// Metrics with the skip enrichment attribute are routed directly to output pipelines,
			// while all other metrics are sent to the enrichment pipeline before output.
			b.addInputRoutingExporter(common.ComponentIDRuntimeInputRoutingConnector, pipelinesWithRuntimeInput),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add runtime service pipeline: %w", err)
		}
	}

	if inputs.prometheus {
		if err := b.AddServicePipeline(ctx, nil, "metrics/input-prometheus",
			b.addPrometheusAppPodsReceiver(),
			b.addPrometheusAppServicesReceiver(opts),
			b.addMemoryLimiterProcessor(),
			b.addDropServiceNameProcessor(),
			b.addSetInstrumentationScopeToPrometheusProcessor(opts),
			// Metrics with the skip enrichment attribute are routed directly to output pipelines,
			// while all other metrics are sent to the enrichment pipeline before output.
			b.addInputRoutingExporter(common.ComponentIDPrometheusInputRoutingConnector, pipelinesWithPrometheusInput),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add prometheus service pipeline: %w", err)
		}
	}

	if inputs.istio {
		if err := b.AddServicePipeline(ctx, nil, "metrics/input-istio",
			b.addPrometheusIstioReceiver(inputs.envoy),
			b.addMemoryLimiterProcessor(),
			b.addDropServiceNameProcessor(),
			b.addIstioNoiseFilterProcessor(),
			b.addSetInstrumentationScopeToIstioProcessor(opts),
			// Metrics with the skip enrichment attribute are routed directly to output pipelines,
			// while all other metrics are sent to the enrichment pipeline before output.
			b.addInputRoutingExporter(common.ComponentIDIstioInputRoutingConnector, pipelinesWithIstioInput),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add istio service pipeline: %w", err)
		}
	}

	// Enrichment pipeline
	if err := b.AddServicePipeline(ctx, nil, enrichmentServicePipelineID,
		b.addInputRoutingReceiver(common.ComponentIDRuntimeInputRoutingConnector, pipelinesWithRuntimeInput, inputs.runtime),
		b.addInputRoutingReceiver(common.ComponentIDPrometheusInputRoutingConnector, pipelinesWithPrometheusInput, inputs.prometheus),
		b.addInputRoutingReceiver(common.ComponentIDIstioInputRoutingConnector, pipelinesWithIstioInput, inputs.istio),
		b.addK8sAttributesProcessor(opts),
		b.addServiceEnrichmentProcessor(),
		b.addEnrichmentRoutingExporter(pipelinesWithRuntimeInput, pipelinesWithPrometheusInput, pipelinesWithIstioInput),
	); err != nil {
		return nil, nil, fmt.Errorf("failed to add enrichment service pipeline: %w", err)
	}

	// Output pipelines
	for _, pipeline := range pipelines {
		outputPipelineID := formatOutputMetricServicePipelineID(&pipeline)
		runtimeInputEnabled := metricpipelineutils.IsRuntimeInputEnabled(pipeline.Spec.Input)
		prometheusInputEnabled := metricpipelineutils.IsPrometheusInputEnabled(pipeline.Spec.Input)
		istioInputEnabled := metricpipelineutils.IsIstioInputEnabled(pipeline.Spec.Input)
		queueSize := common.BatchingMaxQueueSize / len(pipelines)

		if err := b.AddServicePipeline(ctx, &pipeline, outputPipelineID,
			// Receivers
			// Metrics are received from either the enrichment pipeline or directly from input pipelines,
			// depending on whether they have the skip enrichment attribute set.
			b.addEnrichmentRoutingReceiver(pipelinesWithRuntimeInput, pipelinesWithPrometheusInput, pipelinesWithIstioInput),
			b.addInputRoutingReceiver(common.ComponentIDRuntimeInputRoutingConnector, pipelinesWithRuntimeInput, runtimeInputEnabled),
			b.addInputRoutingReceiver(common.ComponentIDPrometheusInputRoutingConnector, pipelinesWithPrometheusInput, prometheusInputEnabled),
			b.addInputRoutingReceiver(common.ComponentIDIstioInputRoutingConnector, pipelinesWithIstioInput, istioInputEnabled),
			// Runtime resource filters
			b.addDropRuntimePodMetricsProcessor(),
			b.addDropRuntimeContainerMetricsProcessor(),
			b.addDropRuntimeNodeMetricsProcessor(),
			b.addDropRuntimeVolumeMetricsProcessor(),
			b.addDropRuntimeDeploymentMetricsProcessor(),
			b.addDropRuntimeDaemonSetMetricsProcessor(),
			b.addDropRuntimeStatefulSetMetricsProcessor(),
			b.addDropRuntimeJobMetricsProcessor(),
			// Diagnostic metric filters
			b.addDropPrometheusDiagnosticMetricsProcessor(),
			b.addDropIstioDiagnosticMetricsProcessor(),
			// Istio envoy metrics
			b.addDropEnvoyMetricsIfDisabledProcessor(),
			// Namespace filters
			b.addRuntimeNamespaceFilterProcessor(),
			b.addPrometheusNamespaceFilterProcessor(),
			b.addIstioNamespaceFilterProcessor(),
			// Generic processors
			b.addInsertClusterAttributesProcessor(opts),
			b.addDropSkipEnrichmentAttributeProcessor(),
			b.addDropKymaAttributesProcessor(),
			b.addUserDefinedTransformProcessor(),
			b.addUserDefinedFilterProcessor(),
			b.addBatchProcessor(), // always last
			// OTLP exporter
			b.addOTLPExporter(queueSize),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add enrichment service pipeline: %w", err)
		}
	}

	return b.Config, b.EnvVars, nil
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

// Input processors

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

// TODO (TeodorSAP):
// The Prometheus receiver sets the service.name attribute by default to the scrape job name,
// which prevents it from being enriched by the service name processor. We currently remove it here,
// but we should investigate configuring the receiver to not set this attribute in the first place.
func (b *Builder) addDropServiceNameProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDResourceDropServiceNameProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return dropServiceNameProcessorConfig()
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

// Enrichment processors

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

// Resource processors

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1alpha1.MetricPipeline) any {
			return common.InsertClusterAttributesProcessorConfig(
				opts.ClusterName, opts.ClusterUID, opts.CloudProvider,
			)
		},
	)
}

func (b *Builder) addDropSkipEnrichmentAttributeProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropSkipEnrichmentAttributeProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.ResourceProcessor{
				Attributes: []common.AttributeAction{
					{
						Action: "delete",
						Key:    common.SkipEnrichmentAttribute,
					},
				},
			}
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.DropKymaAttributesProcessorConfig()
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if len(mp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(mp.Spec.Transforms)
			transformProcessor := common.MetricTransformProcessorConfig(transformStatements)

			return transformProcessor
		},
	)
}

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if mp.Spec.Filters == nil {
				return nil // No filters, no processor needed
			}

			return common.FilterSpecsToMetricFilterProcessorConfig(mp.Spec.Filters)
		},
	)
}

// Namespace filter processors

func (b *Builder) addRuntimeNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceRuntime)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsRuntimeInputEnabled(input) || !shouldFilterByNamespace(input.Runtime.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.Runtime.Namespaces, inputSourceEquals(common.InputSourceRuntime))
		},
	)
}

func (b *Builder) addPrometheusNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourcePrometheus)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsPrometheusInputEnabled(input) || !shouldFilterByNamespace(input.Prometheus.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.Prometheus.Namespaces, common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus))
		},
	)
}

func (b *Builder) addIstioNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1alpha1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceIstio)
		},
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsIstioInputEnabled(input) || !shouldFilterByNamespace(input.Istio.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessorConfig(input.Istio.Namespaces, inputSourceEquals(common.InputSourceIstio))
		},
	)
}

func filterByNamespaceProcessorConfig(namespaceSelector *telemetryv1alpha1.NamespaceSelector, inputSourceCondition string) *common.FilterProcessor {
	var filterExpressions []string

	if len(namespaceSelector.Exclude) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Exclude)
		excludeNamespacesExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(namespacesConditions...))
		filterExpressions = append(filterExpressions, excludeNamespacesExpr)
	}

	if len(namespaceSelector.Include) > 0 {
		namespacesConditions := namespacesConditions(namespaceSelector.Include)
		includeNamespacesExpr := common.JoinWithAnd(
			inputSourceCondition,
			common.ResourceAttributeIsNotNil(common.K8sNamespaceName),
			common.Not(common.JoinWithOr(namespacesConditions...)),
		)
		filterExpressions = append(filterExpressions, includeNamespacesExpr)
	}

	return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
		Metric: filterExpressions,
	})
}

func namespacesConditions(namespaces []string) []string {
	var conditions []string
	for _, ns := range namespaces {
		conditions = append(conditions, common.NamespaceEquals(ns))
	}

	return conditions
}

// Runtime resource filter processors

func (b *Builder) addDropRuntimePodMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimePodMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimePodInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.pod.*"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeContainerMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeContainerMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeContainerInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "(^k8s.container.*)|(^container.*)"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeNodeMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeNodeMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeNodeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.node.*"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeVolumeMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeVolumeMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeVolumeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.volume.*"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeDeploymentMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeDeploymentMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDeploymentInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.deployment.*"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeDaemonSetMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeDaemonSetMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDaemonSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.daemonset.*"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeStatefulSetMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeStatefulSetMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeStatefulSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.statefulset.*"),
				)},
			})
		},
	)
}

func (b *Builder) addDropRuntimeJobMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeJobMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeJobInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					inputSourceEquals(common.InputSourceRuntime),
					common.IsMatch("name", "^k8s.job.*"),
				)},
			})
		},
	)
}

// Diagnostic metric filter processors

func (b *Builder) addDropPrometheusDiagnosticMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropPrometheusDiagnosticMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) || metricpipelineutils.IsPrometheusDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return dropDiagnosticMetricsFilterProcessorConfig(common.InputSourcePrometheus)
		},
	)
}

func (b *Builder) addDropIstioDiagnosticMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIstioDiagnosticMetricsProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) || metricpipelineutils.IsIstioDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return dropDiagnosticMetricsFilterProcessorConfig(common.InputSourceIstio)
		},
	)
}

func dropDiagnosticMetricsFilterProcessorConfig(inputSource common.InputSourceType) *common.FilterProcessor {
	var filterExpressions []string

	inputSourceCondition := inputSourceEquals(inputSource)
	metricNameConditions := nameConditions(diagnosticMetricNames)
	excludeScrapeMetricsExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(metricNameConditions...))
	filterExpressions = append(filterExpressions, excludeScrapeMetricsExpr)

	return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
		Metric: filterExpressions,
	})
}

func nameConditions(names []string) []string {
	var nameConditions []string
	for _, name := range names {
		nameConditions = append(nameConditions, common.NameAttributeEquals(name))
	}

	return nameConditions
}

// Istio envoy metrics

func (b *Builder) addDropEnvoyMetricsIfDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropEnvoyMetricsIfDisabledProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) && metricpipelineutils.IsEnvoyMetricsEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
				Metric: []string{common.JoinWithAnd(
					common.IsMatch("name", "^envoy_.*"),
					common.ScopeNameEquals(common.InstrumentationScopeIstio),
				)},
			})
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
func (b *Builder) addOTLPExporter(queueSize int) buildComponentFunc {
	return b.AddExporter(
		formatOTLPExporterID,
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				mp.Spec.Output.OTLP,
				mp.Name,
				queueSize,
				common.SignalTypeMetric,
			)

			return otlpExporterBuilder.OTLPExporterConfig(ctx)
		},
	)
}

// Connector builders

func (b *Builder) addInputRoutingExporter(componentID string, outputPipelines []telemetryv1alpha1.MetricPipeline) buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(componentID),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return inputRoutingConnectorConfig(formatOutputPipelineIDs(outputPipelines)), nil, nil
		},
	)
}

func (b *Builder) addInputRoutingReceiver(componentID string, outputPipelines []telemetryv1alpha1.MetricPipeline, inputEnabled bool) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(componentID),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if !inputEnabled {
				return nil
			}

			return inputRoutingConnectorConfig(formatOutputPipelineIDs(outputPipelines))
		},
	)
}

func (b *Builder) addEnrichmentRoutingExporter(runtimePipelines, prometheusPipelines, istioPipelines []telemetryv1alpha1.MetricPipeline) buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(common.ComponentIDEnrichmentRoutingConnector),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return enrichmentRoutingConnectorConfig(runtimePipelines, prometheusPipelines, istioPipelines), nil, nil
		},
	)
}

func (b *Builder) addEnrichmentRoutingReceiver(runtimePipelines, prometheusPipelines, istioPipelines []telemetryv1alpha1.MetricPipeline) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDEnrichmentRoutingConnector),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if len(runtimePipelines) == 0 && len(prometheusPipelines) == 0 && len(istioPipelines) == 0 {
				return nil
			}

			return enrichmentRoutingConnectorConfig(runtimePipelines, prometheusPipelines, istioPipelines)
		},
	)
}

func enrichmentRoutingConnectorConfig(runtimePipelines, prometheusPipelines, istioPipelines []telemetryv1alpha1.MetricPipeline) common.RoutingConnector {
	tableEntries := []common.RoutingConnectorTableEntry{}

	if len(runtimePipelines) > 0 {
		tableEntries = append(tableEntries, enrichmentRoutingConnectorTableEntry(runtimePipelines, inputSourceEquals(common.InputSourceRuntime)))
	}

	if len(prometheusPipelines) > 0 {
		tableEntries = append(tableEntries, enrichmentRoutingConnectorTableEntry(prometheusPipelines, common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus)))
	}

	if len(istioPipelines) > 0 {
		tableEntries = append(tableEntries, enrichmentRoutingConnectorTableEntry(istioPipelines, common.ScopeNameEquals(common.InstrumentationScopeIstio)))
	}

	return common.RoutingConnector{
		ErrorMode: "ignore",
		Table:     tableEntries,
	}
}

func enrichmentRoutingConnectorTableEntry(pipelines []telemetryv1alpha1.MetricPipeline, routingCondition string) common.RoutingConnectorTableEntry {
	return common.RoutingConnectorTableEntry{
		Context:   "metric",
		Statement: fmt.Sprintf("route() where %s", routingCondition),
		Pipelines: formatOutputPipelineIDs(pipelines),
	}
}

func inputRoutingConnectorConfig(outputPipelineIDs []string) common.RoutingConnector {
	return common.RoutingConnector{
		DefaultPipelines: []string{enrichmentServicePipelineID},
		ErrorMode:        "ignore",
		Table: []common.RoutingConnectorTableEntry{
			{
				Statement: fmt.Sprintf("route() where attributes[\"%s\"] == \"true\"", common.SkipEnrichmentAttribute),
				Pipelines: outputPipelineIDs,
			},
		},
	}
}

// Helper functions for formatting IDs

func formatOutputPipelineIDs(pipelines []telemetryv1alpha1.MetricPipeline) []string {
	var ids []string
	for i := range pipelines {
		ids = append(ids, fmt.Sprintf("metrics/output-%s", pipelines[i].Name))
	}

	return ids
}

func formatOutputMetricServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/output-%s", mp.Name)
}

func formatOTLPExporterID(pipeline *telemetryv1alpha1.MetricPipeline) string {
	return common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
}

func formatNamespaceFilterID(pipelineName string, inputSourceType common.InputSourceType) string {
	return fmt.Sprintf(common.ComponentIDNamespacePerInputFilterProcessor, pipelineName, inputSourceType)
}

// Helper functions for getting pipelines by input source

func getPipelinesWithRuntimeInput(pipelines []telemetryv1alpha1.MetricPipeline) []telemetryv1alpha1.MetricPipeline {
	var result []telemetryv1alpha1.MetricPipeline

	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) {
			result = append(result, pipelines[i])
		}
	}

	return result
}

func getPipelinesWithPrometheusInput(pipelines []telemetryv1alpha1.MetricPipeline) []telemetryv1alpha1.MetricPipeline {
	var result []telemetryv1alpha1.MetricPipeline

	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsPrometheusInputEnabled(input) {
			result = append(result, pipelines[i])
		}
	}

	return result
}

func getPipelinesWithIstioInput(pipelines []telemetryv1alpha1.MetricPipeline) []telemetryv1alpha1.MetricPipeline {
	var result []telemetryv1alpha1.MetricPipeline

	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsIstioInputEnabled(input) {
			result = append(result, pipelines[i])
		}
	}

	return result
}

func inputSourceEquals(inputSourceType common.InputSourceType) string {
	return common.ScopeNameEquals(common.InstrumentationScope[inputSourceType])
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

func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

// Processor configuration functions (merged from processors.go)

func dropServiceNameProcessorConfig() *common.ResourceProcessor {
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

func dropNonPVCVolumesMetricsProcessorConfig() *common.FilterProcessor {
	return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
		Metric: []string{common.JoinWithAnd(
			// identify volume metrics by checking existence of "k8s.volume.name" resource attribute
			common.ResourceAttributeIsNotNil("k8s.volume.name"),
			common.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
		)},
	})
}

func dropVirtualNetworkInterfacesProcessorConfig() *common.FilterProcessor {
	return common.MetricFilterProcessorConfig(common.FilterProcessorMetrics{
		Datapoint: []string{common.JoinWithAnd(
			common.IsMatch("metric.name", "^k8s.node.network.*"),
			common.Not(common.IsMatch("attributes[\"interface\"]", "^(eth|en).*")),
		)},
	})
}

func metricNameConditionsWithIsMatch(metrics []string) []string {
	var conditions []string

	for _, m := range metrics {
		condition := common.IsMatch("metric.name", fmt.Sprintf("^k8s.%s.*", m))
		conditions = append(conditions, condition)
	}

	return conditions
}

func formatUserDefinedTransformProcessorID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedTransformProcessor, mp.Name)
}

func formatUserDefinedFilterProcessorID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf(common.ComponentIDUserDefinedFilterProcessor, mp.Name)
}
