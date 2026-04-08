package metricagent

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
	telemetryutils "github.com/kyma-project/telemetry-manager/internal/utils/telemetry"
)

const enrichmentServicePipelineID = "metrics/enrichment-conditional"

var diagnosticMetricNames = []string{"up", "scrape_duration_seconds", "scrape_samples_scraped", "scrape_samples_post_metric_relabeling", "scrape_series_added"}

type buildComponentFunc = common.BuildComponentFunc[*telemetryv1beta1.MetricPipeline]

type Builder struct {
	common.ComponentBuilder[*telemetryv1beta1.MetricPipeline]

	Reader client.Reader
}

type BuildOptions struct {
	Cluster common.ClusterOptions

	// IstioActive indicates whether Istio is installed in the cluster.
	IstioActive                 bool
	IstioCertPath               string
	InstrumentationScopeVersion string
	AgentNamespace              string
	Enrichments                 *operatorv1beta1.EnrichmentSpec
	// ServiceEnrichment specifies the service enrichment strategy to be used (temporary)
	ServiceEnrichment string
	// VpaActive indicates whether VPA is active (VPA CRD exists and VPA is enabled via annotation in Telemetry CR).
	VpaActive bool
	// CollectionIntervals contains the resolved collection intervals for each pull-based metric input type.
	CollectionIntervals telemetryutils.MetricCollectionIntervals
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

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1beta1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	// Sort pipelines to ensure consistent order and checksum for generated ConfigMap
	slices.SortFunc(pipelines, func(a, b telemetryv1beta1.MetricPipeline) int {
		return strings.Compare(a.Name, b.Name)
	})

	b.Config = common.NewConfig()
	if opts.VpaActive {
		b.Config.DisableGoMemLimit()
	}

	b.AddExtension(common.ComponentIDK8sLeaderElectorExtension,
		common.K8sLeaderElectorExtensionConfig{
			AuthType:       "serviceAccount",
			LeaseName:      common.K8sLeaderElectorK8sCluster,
			LeaseNamespace: opts.AgentNamespace,
		},
		nil,
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
			b.addKubeletStatsReceiver(inputs.runtimeResources, opts.CollectionIntervals.Runtime),
			b.addK8sClusterReceiver(inputs.runtimeResources, opts.CollectionIntervals.Runtime),
			b.addMemoryLimiterProcessor(),
			b.addFilterDropNonPVCVolumesMetricsProcessor(inputs.runtimeResources),
			b.addFilterDropVirtualNetworkInterfacesProcessor(),
			b.addDropServiceNameProcessor(),
			b.addInsertSkipEnrichmentAttributeProcessor(),
			b.addSetInstrumentationScopeToRuntimeProcessor(opts),
			b.addSetKymaInputNameProcessor(common.InputSourceRuntime),
			// Metrics with the skip enrichment attribute are routed directly to output pipelines,
			// while all other metrics are sent to the enrichment pipeline before output.
			b.addExporterForInputRouter(common.ComponentIDRuntimeInputRoutingConnector, pipelinesWithRuntimeInput),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add runtime service pipeline: %w", err)
		}
	}

	if inputs.prometheus {
		if err := b.AddServicePipeline(ctx, nil, "metrics/input-prometheus",
			b.addPrometheusAppPodsReceiver(opts.CollectionIntervals.Prometheus),
			b.addPrometheusAppServicesReceiver(opts, opts.CollectionIntervals.Prometheus),
			b.addMemoryLimiterProcessor(),
			b.addDropServiceNameProcessor(),
			b.addSetInstrumentationScopeToPrometheusProcessor(opts),
			b.addSetKymaInputNameProcessor(common.InputSourcePrometheus),
			// Metrics with the skip enrichment attribute are routed directly to output pipelines,
			// while all other metrics are sent to the enrichment pipeline before output.
			b.addExporterForInputRouter(common.ComponentIDPrometheusInputRoutingConnector, pipelinesWithPrometheusInput),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add prometheus service pipeline: %w", err)
		}
	}

	if inputs.istio {
		if err := b.AddServicePipeline(ctx, nil, "metrics/input-istio",
			b.addPrometheusIstioReceiver(inputs.envoy, opts.CollectionIntervals.Istio),
			b.addMemoryLimiterProcessor(),
			b.addDropServiceNameProcessor(),
			b.addIstioNoiseFilterProcessor(),
			b.addSetInstrumentationScopeToIstioProcessor(opts),
			b.addSetKymaInputNameProcessor(common.InputSourceIstio),
			// Metrics with the skip enrichment attribute are routed directly to output pipelines,
			// while all other metrics are sent to the enrichment pipeline before output.
			b.addExporterForInputRouter(common.ComponentIDIstioInputRoutingConnector, pipelinesWithIstioInput),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add istio service pipeline: %w", err)
		}
	}

	// Enrichment pipeline
	if err := b.AddServicePipeline(ctx, nil, enrichmentServicePipelineID,
		b.addReceiverForInputRouter(common.ComponentIDRuntimeInputRoutingConnector, pipelinesWithRuntimeInput, inputs.runtime),
		b.addReceiverForInputRouter(common.ComponentIDPrometheusInputRoutingConnector, pipelinesWithPrometheusInput, inputs.prometheus),
		b.addReceiverForInputRouter(common.ComponentIDIstioInputRoutingConnector, pipelinesWithIstioInput, inputs.istio),
		b.addDropUnknownServiceNameProcessor(opts),
		b.addK8sAttributesProcessor(opts),
		b.addServiceEnrichmentProcessor(opts),
		b.addExporterForEnrichmentRouter(pipelinesWithRuntimeInput, pipelinesWithPrometheusInput, pipelinesWithIstioInput),
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

		if shouldEnableOAuth2(&pipeline) {
			if err := b.addOAuth2Extension(ctx, &pipeline); err != nil {
				return nil, nil, err
			}
		}

		if err := b.AddServicePipeline(ctx, &pipeline, outputPipelineID,
			// Receivers
			// Metrics are received from either the enrichment pipeline or directly from input pipelines,
			// depending on whether they have the skip enrichment attribute set.
			b.addReceiverForEnrichmentRouter(pipelinesWithRuntimeInput, pipelinesWithPrometheusInput, pipelinesWithIstioInput),
			b.addReceiverForInputRouter(common.ComponentIDRuntimeInputRoutingConnector, pipelinesWithRuntimeInput, runtimeInputEnabled),
			b.addReceiverForInputRouter(common.ComponentIDPrometheusInputRoutingConnector, pipelinesWithPrometheusInput, prometheusInputEnabled),
			b.addReceiverForInputRouter(common.ComponentIDIstioInputRoutingConnector, pipelinesWithIstioInput, istioInputEnabled),
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
			// Kyma attributes are dropped before user-defined transform and filter processors
			// to prevent user access to internal attributes.
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

func (b *Builder) addK8sClusterReceiver(runtimeResources runtimeResourceSources, collectionInterval time.Duration) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDK8sClusterReceiver),
		func(*telemetryv1beta1.MetricPipeline) any {
			return k8sClusterReceiver(runtimeResources, collectionInterval)
		},
	)
}

func (b *Builder) addKubeletStatsReceiver(runtimeResources runtimeResourceSources, collectionInterval time.Duration) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDKubeletStatsReceiver),
		func(*telemetryv1beta1.MetricPipeline) any {
			return kubeletStatsReceiver(runtimeResources, collectionInterval)
		},
	)
}

func (b *Builder) addPrometheusAppPodsReceiver(collectionInterval time.Duration) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDPrometheusAppPodsReceiver),
		func(*telemetryv1beta1.MetricPipeline) any {
			return prometheusPodsReceiverConfig(collectionInterval)
		},
	)
}

func (b *Builder) addPrometheusAppServicesReceiver(opts BuildOptions, collectionInterval time.Duration) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDPrometheusAppServicesReceiver),
		func(*telemetryv1beta1.MetricPipeline) any {
			return prometheusServicesReceiverConfig(opts, collectionInterval)
		},
	)
}

func (b *Builder) addPrometheusIstioReceiver(envoyMetricsEnabled bool, collectionInterval time.Duration) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDPrometheusIstioReceiver),
		func(*telemetryv1beta1.MetricPipeline) any {
			return prometheusIstioReceiverConfig(envoyMetricsEnabled, collectionInterval)
		},
	)
}

// Input processors

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.MemoryLimiterConfig{
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
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !runtimeResources.volume {
				return nil
			}

			return dropNonPVCVolumesMetricsProcessor()
		},
	)
}

func (b *Builder) addFilterDropVirtualNetworkInterfacesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDFilterDropVirtualNetworkInterfacesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return dropVirtualNetworkInterfacesProcessor()
		},
	)
}

// TODO (TeodorSAP):
// The Prometheus receiver sets the service.name attribute by default to the scrape job name,
// which prevents it from being enriched by the service name processor. We currently remove it here,
// but we should investigate configuring the receiver to not set this attribute in the first place.
// (4 Dec. 2025, TeodorSAP): No solution found yet.
func (b *Builder) addDropServiceNameProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropServiceNameProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return dropServiceNameProcessor()
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToRuntimeProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeRuntimeProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return common.InstrumentationScopeProcessor(opts.InstrumentationScopeVersion, common.InputSourceRuntime, common.InputSourceK8sCluster)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToPrometheusProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopePrometheusProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return common.InstrumentationScopeProcessor(opts.InstrumentationScopeVersion, common.InputSourcePrometheus)
		},
	)
}

func (b *Builder) addSetInstrumentationScopeToIstioProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDSetInstrumentationScopeIstioProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return common.InstrumentationScopeProcessor(opts.InstrumentationScopeVersion, common.InputSourceIstio)
		},
	)
}

func (b *Builder) addSetKymaInputNameProcessor(inputSource common.InputSourceType) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.InputName[inputSource]),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.KymaInputNameProcessorStatements(inputSource)
			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addInsertSkipEnrichmentAttributeProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertSkipEnrichmentAttributeProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return insertSkipEnrichmentAttributeProcessor()
		},
	)
}

func (b *Builder) addIstioNoiseFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDIstioNoiseFilterProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.IstioNoiseFilterProcessorConfig{}
		},
	)
}

// Enrichment processors

func (b *Builder) addDropUnknownServiceNameProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropUnknownServiceNameProcessor),
		func(tp *telemetryv1beta1.MetricPipeline) any {
			if opts.ServiceEnrichment != commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // Kyma legacy enrichment selected, skip this processor
			}

			return common.MetricTransformProcessor(common.DropUnknownServiceNameProcessorStatements())
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			useOTelServiceEnrichment := opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel
			return common.K8sAttributesProcessor(opts.Enrichments, useOTelServiceEnrichment)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if opts.ServiceEnrichment == commonresources.AnnotationValueTelemetryServiceEnrichmentOtel {
				return nil // OTel service enrichment selected, skip this processor
			}

			return common.ResolveServiceName()
		},
	)
}

// Resource processors

func (b *Builder) addInsertClusterAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDInsertClusterAttributesProcessor),
		func(tp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.InsertClusterAttributesProcessorStatements(opts.Cluster)
			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addDropSkipEnrichmentAttributeProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropSkipEnrichmentAttributeProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := []common.TransformProcessorStatements{{
				Statements: []string{
					"delete_key(resource.attributes, \"io.kyma-project.telemetry.skip_enrichment\")",
				},
			}}

			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addDropKymaAttributesProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropKymaAttributesProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			transformStatements := common.DropKymaAttributesProcessorStatements()
			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedTransformProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedTransformProcessorID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if len(mp.Spec.Transforms) == 0 {
				return nil // No transforms, no processor needed
			}

			transformStatements := common.TransformSpecsToProcessorStatements(mp.Spec.Transforms)

			return common.MetricTransformProcessor(transformStatements)
		},
	)
}

func (b *Builder) addUserDefinedFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		formatUserDefinedFilterProcessorID,
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if mp.Spec.Filters == nil {
				return nil // No filters, no processor needed
			}

			return common.MetricFilterProcessor(mp.Spec.Filters)
		},
	)
}

// Namespace filter processors

func (b *Builder) addRuntimeNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1beta1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceRuntime)
		},
		func(mp *telemetryv1beta1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsRuntimeInputEnabled(input) || !shouldFilterByNamespace(input.Runtime.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessor(input.Runtime.Namespaces, common.KymaInputNameEquals(common.InputSourceRuntime))
		},
	)
}

func (b *Builder) addPrometheusNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1beta1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourcePrometheus)
		},
		func(mp *telemetryv1beta1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsPrometheusInputEnabled(input) || !shouldFilterByNamespace(input.Prometheus.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessor(input.Prometheus.Namespaces, common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus))
		},
	)
}

func (b *Builder) addIstioNamespaceFilterProcessor() buildComponentFunc {
	return b.AddProcessor(
		func(mp *telemetryv1beta1.MetricPipeline) string {
			return formatNamespaceFilterID(mp.Name, common.InputSourceIstio)
		},
		func(mp *telemetryv1beta1.MetricPipeline) any {
			input := mp.Spec.Input
			if !metricpipelineutils.IsIstioInputEnabled(input) || !shouldFilterByNamespace(input.Istio.Namespaces) {
				return nil
			}

			return filterByNamespaceProcessor(input.Istio.Namespaces, common.KymaInputNameEquals(common.InputSourceIstio))
		},
	)
}

func filterByNamespaceProcessor(namespaceSelector *telemetryv1beta1.NamespaceSelector, inputSourceCondition string) *common.FilterProcessorConfig {
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

	return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
		{
			Conditions: filterExpressions,
		},
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
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimePodInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.pod.*"),
					)},
				},
			})
		},
	)
}

func (b *Builder) addDropRuntimeContainerMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeContainerMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeContainerInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor(
				[]telemetryv1beta1.FilterSpec{
					{
						Conditions: []string{common.JoinWithAnd(
							common.KymaInputNameEquals(common.InputSourceRuntime),
							common.IsMatch("metric.name", "(^k8s.container.*)|(^container.*)"),
						)},
					},
				})
		},
	)
}

func (b *Builder) addDropRuntimeNodeMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeNodeMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeNodeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.node.*"),
					)},
				},
			})
		},
	)
}

func (b *Builder) addDropRuntimeVolumeMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeVolumeMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeVolumeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.volume.*"),
					)},
				},
			})
		},
	)
}

func (b *Builder) addDropRuntimeDeploymentMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeDeploymentMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDeploymentInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.deployment.*"),
					)},
				},
			})
		},
	)
}

func (b *Builder) addDropRuntimeDaemonSetMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeDaemonSetMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeDaemonSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.daemonset.*"),
					)},
				},
			})
		},
	)
}

func (b *Builder) addDropRuntimeStatefulSetMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeStatefulSetMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeStatefulSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.statefulset.*"),
					)},
				},
			})
		},
	)
}

func (b *Builder) addDropRuntimeJobMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropRuntimeJobMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) || metricpipelineutils.IsRuntimeJobInputEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.KymaInputNameEquals(common.InputSourceRuntime),
						common.IsMatch("metric.name", "^k8s.job.*"),
					)},
				},
			})
		},
	)
}

// Diagnostic metric filter processors

func (b *Builder) addDropPrometheusDiagnosticMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropPrometheusDiagnosticMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) || metricpipelineutils.IsPrometheusDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return dropDiagnosticMetricsFilterProcessor(common.InputSourcePrometheus)
		},
	)
}

func (b *Builder) addDropIstioDiagnosticMetricsProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropIstioDiagnosticMetricsProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) || metricpipelineutils.IsIstioDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return dropDiagnosticMetricsFilterProcessor(common.InputSourceIstio)
		},
	)
}

func dropDiagnosticMetricsFilterProcessor(inputSource common.InputSourceType) *common.FilterProcessorConfig {
	var filterExpressions []string

	inputSourceCondition := common.KymaInputNameEquals(inputSource)
	metricNameConditions := nameConditions(diagnosticMetricNames)
	excludeScrapeMetricsExpr := common.JoinWithAnd(inputSourceCondition, common.JoinWithOr(metricNameConditions...))
	filterExpressions = append(filterExpressions, excludeScrapeMetricsExpr)

	return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
		{
			Conditions: filterExpressions,
		},
	})
}

func nameConditions(names []string) []string {
	var nameConditions []string
	for _, name := range names {
		nameConditions = append(nameConditions, common.MetricNameAttributeEquals(name))
	}

	return nameConditions
}

// Istio envoy metrics

func (b *Builder) addDropEnvoyMetricsIfDisabledProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDDropEnvoyMetricsIfDisabledProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) && metricpipelineutils.IsEnvoyMetricsEnabled(mp.Spec.Input) {
				return nil
			}

			return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
				{
					Conditions: []string{common.JoinWithAnd(
						common.IsMatch("metric.name", "^envoy_.*"),
						common.KymaInputNameEquals(common.InputSourceIstio),
					)},
				},
			})
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addBatchProcessor() buildComponentFunc {
	return b.AddProcessor(
		b.StaticComponentID(common.ComponentIDBatchProcessor),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			return &common.BatchProcessorConfig{
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
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
				b.Reader,
				mp.Spec.Output.OTLP,
				common.MetricPipelineRef(mp),
				queueSize,
			)

			return otlpExporterBuilder.OTLPExporter(ctx)
		},
	)
}

// Connector builders

func (b *Builder) addExporterForInputRouter(componentID string, outputPipelines []telemetryv1beta1.MetricPipeline) buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(componentID),
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			return inputRoutingConnector(formatOutputPipelineIDs(outputPipelines)), nil, nil
		},
	)
}

func (b *Builder) addReceiverForInputRouter(componentID string, outputPipelines []telemetryv1beta1.MetricPipeline, inputEnabled bool) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(componentID),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if !inputEnabled {
				return nil
			}

			return inputRoutingConnector(formatOutputPipelineIDs(outputPipelines))
		},
	)
}

func (b *Builder) addExporterForEnrichmentRouter(runtimePipelines, prometheusPipelines, istioPipelines []telemetryv1beta1.MetricPipeline) buildComponentFunc {
	return b.AddExporter(
		b.StaticComponentID(common.ComponentIDEnrichmentRoutingConnector),
		func(ctx context.Context, mp *telemetryv1beta1.MetricPipeline) (any, common.EnvVars, error) {
			return enrichmentRoutingConnector(runtimePipelines, prometheusPipelines, istioPipelines), nil, nil
		},
	)
}

func (b *Builder) addReceiverForEnrichmentRouter(runtimePipelines, prometheusPipelines, istioPipelines []telemetryv1beta1.MetricPipeline) buildComponentFunc {
	return b.AddReceiver(
		b.StaticComponentID(common.ComponentIDEnrichmentRoutingConnector),
		func(mp *telemetryv1beta1.MetricPipeline) any {
			if len(runtimePipelines) == 0 && len(prometheusPipelines) == 0 && len(istioPipelines) == 0 {
				return nil
			}

			return enrichmentRoutingConnector(runtimePipelines, prometheusPipelines, istioPipelines)
		},
	)
}

func enrichmentRoutingConnector(runtimePipelines, prometheusPipelines, istioPipelines []telemetryv1beta1.MetricPipeline) common.RoutingConnectorConfig {
	tableEntries := []common.RoutingConnectorTableEntry{}

	if len(runtimePipelines) > 0 {
		tableEntries = append(tableEntries, enrichmentRoutingConnectorTableEntry(runtimePipelines, common.KymaInputNameEquals(common.InputSourceRuntime)))
	}

	if len(prometheusPipelines) > 0 {
		tableEntries = append(tableEntries, enrichmentRoutingConnectorTableEntry(prometheusPipelines, common.KymaInputNameEquals(common.InputSourcePrometheus)))
	}

	if len(istioPipelines) > 0 {
		tableEntries = append(tableEntries, enrichmentRoutingConnectorTableEntry(istioPipelines, common.KymaInputNameEquals(common.InputSourceIstio)))
	}

	return common.RoutingConnectorConfig{
		ErrorMode: "ignore",
		Table:     tableEntries,
	}
}

func enrichmentRoutingConnectorTableEntry(pipelines []telemetryv1beta1.MetricPipeline, routingCondition string) common.RoutingConnectorTableEntry {
	return common.RoutingConnectorTableEntry{
		Context:   "metric",
		Statement: fmt.Sprintf("route() where %s", routingCondition),
		Pipelines: formatOutputPipelineIDs(pipelines),
	}
}

func inputRoutingConnector(outputPipelineIDs []string) common.RoutingConnectorConfig {
	return common.RoutingConnectorConfig{
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

// Authentication extensions

func (b *Builder) addOAuth2Extension(ctx context.Context, pipeline *telemetryv1beta1.MetricPipeline) error {
	oauth2ExtensionID := common.OAuth2ExtensionID(common.MetricPipelineRef(pipeline))

	oauth2ExtensionConfig, oauth2ExtensionEnvVars, err := common.NewOAuth2ExtensionConfigBuilder(
		b.Reader,
		pipeline.Spec.Output.OTLP.Authentication.OAuth2,
		common.MetricPipelineRef(pipeline),
	).OAuth2Extension(ctx)
	if err != nil {
		return fmt.Errorf("failed to build OAuth2 extension for pipeline %s: %w", pipeline.Name, err)
	}

	b.AddExtension(oauth2ExtensionID, oauth2ExtensionConfig, oauth2ExtensionEnvVars)

	return nil
}

// Helper functions for formatting IDs

func formatOutputPipelineIDs(pipelines []telemetryv1beta1.MetricPipeline) []string {
	var ids []string
	for i := range pipelines {
		ids = append(ids, fmt.Sprintf("metrics/output-%s", pipelines[i].Name))
	}

	return ids
}

func formatOutputMetricServicePipelineID(mp *telemetryv1beta1.MetricPipeline) string {
	return fmt.Sprintf("metrics/output-%s", mp.Name)
}

func formatOTLPExporterID(pipeline *telemetryv1beta1.MetricPipeline) string {
	return common.OTLPExporterID(pipeline.Spec.Output.OTLP.Protocol, common.MetricPipelineRef(pipeline))
}

func formatNamespaceFilterID(pipelineName string, inputSourceType common.InputSourceType) string {
	return common.ComponentIDNamespacePerInputFilterProcessor(pipelineName, inputSourceType)
}

// Helper functions for getting pipelines by input source

func getPipelinesWithRuntimeInput(pipelines []telemetryv1beta1.MetricPipeline) []telemetryv1beta1.MetricPipeline {
	var result []telemetryv1beta1.MetricPipeline

	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) {
			result = append(result, pipelines[i])
		}
	}

	return result
}

func getPipelinesWithPrometheusInput(pipelines []telemetryv1beta1.MetricPipeline) []telemetryv1beta1.MetricPipeline {
	var result []telemetryv1beta1.MetricPipeline

	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsPrometheusInputEnabled(input) {
			result = append(result, pipelines[i])
		}
	}

	return result
}

func getPipelinesWithIstioInput(pipelines []telemetryv1beta1.MetricPipeline) []telemetryv1beta1.MetricPipeline {
	var result []telemetryv1beta1.MetricPipeline

	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsIstioInputEnabled(input) {
			result = append(result, pipelines[i])
		}
	}

	return result
}

// Helper functions for determining what should be enabled

func shouldEnableRuntimeMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimePodMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimePodInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeContainerMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeNodeMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeVolumeMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeStatefulSetMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeDeploymentMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeDaemonSetMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableRuntimeJobMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsRuntimeInputEnabled(input) && metricpipelineutils.IsRuntimeJobInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnablePrometheusMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsPrometheusInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableIstioMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsIstioInputEnabled(input) {
			return true
		}
	}

	return false
}

func shouldEnableEnvoyMetricsScraping(pipelines []telemetryv1beta1.MetricPipeline) bool {
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if metricpipelineutils.IsIstioInputEnabled(input) && metricpipelineutils.IsEnvoyMetricsEnabled(input) {
			return true
		}
	}

	return false
}

func shouldFilterByNamespace(namespaceSelector *telemetryv1beta1.NamespaceSelector) bool {
	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
}

func shouldEnableOAuth2(tp *telemetryv1beta1.MetricPipeline) bool {
	return tp.Spec.Output.OTLP.Authentication != nil && tp.Spec.Output.OTLP.Authentication.OAuth2 != nil
}

// Processor configuration functions (merged from processors.go)

func dropServiceNameProcessor() *common.TransformProcessorConfig {
	return common.MetricTransformProcessor(
		[]common.TransformProcessorStatements{{
			Statements: []string{
				"delete_key(resource.attributes, \"service.name\")",
			},
		}},
	)
}

func insertSkipEnrichmentAttributeProcessor() *common.TransformProcessorConfig {
	metricsToSkipEnrichment := []string{
		"node",
		"statefulset",
		"daemonset",
		"deployment",
		"job",
	}

	return common.MetricTransformProcessor([]common.TransformProcessorStatements{{
		Conditions: metricNameConditionsWithIsMatch(metricsToSkipEnrichment),
		Statements: []string{fmt.Sprintf("set(resource.attributes[\"%s\"], \"true\")", common.SkipEnrichmentAttribute)},
	}})
}

func dropNonPVCVolumesMetricsProcessor() *common.FilterProcessorConfig {
	return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
		{
			Conditions: []string{common.JoinWithAnd(
				common.ResourceAttributeIsNotNil("k8s.volume.name"),
				common.ResourceAttributeNotEquals("k8s.volume.type", "persistentVolumeClaim"),
			)},
		},
	})
}

func dropVirtualNetworkInterfacesProcessor() *common.FilterProcessorConfig {
	return common.MetricFilterProcessor([]telemetryv1beta1.FilterSpec{
		{
			Conditions: []string{common.JoinWithAnd(
				common.IsMatch("metric.name", "^k8s.node.network.*"),
				common.Not(common.IsMatch("datapoint.attributes[\"interface\"]", "^(eth|en).*")),
			)},
		},
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

func formatUserDefinedTransformProcessorID(mp *telemetryv1beta1.MetricPipeline) string {
	ref := common.MetricPipelineRef(mp)
	return common.ComponentIDUserDefinedTransformProcessor(ref.TypePrefix(), ref.Name())
}

func formatUserDefinedFilterProcessorID(mp *telemetryv1beta1.MetricPipeline) string {
	ref := common.MetricPipelineRef(mp)
	return common.ComponentIDUserDefinedFilterProcessor(ref.TypePrefix(), ref.Name())
}
