package metricgateway

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	metricpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/metricpipeline"
)

const (
	maxQueueSize = 256 // Maximum number of batches kept in memory before dropping
)

type Builder struct {
	Reader client.Reader

	config  *common.Config
	envVars common.EnvVars
}

type BuildOptions struct {
	GatewayNamespace            string
	InstrumentationScopeVersion string
	ClusterName                 string
	ClusterUID                  string
	CloudProvider               string
	Enrichments                 *operatorv1alpha1.EnrichmentSpec
}

func (b *Builder) Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, opts BuildOptions) (*common.Config, common.EnvVars, error) {
	b.config = &common.Config{
		Base:       common.BaseConfig(common.WithK8sLeaderElector("serviceAccount", "telemetry-metric-gateway-kymastats", opts.GatewayNamespace)),
		Receivers:  make(map[string]any),
		Processors: make(map[string]any),
		Exporters:  make(map[string]any),
		Connectors: make(map[string]any),
	}
	b.envVars = make(common.EnvVars)

	// Iterate over each MetricPipeline CR and enrich the config with pipeline-specific components
	// queueSize := maxQueueSize / len(pipelines)

	for i := range pipelines {
		if err := b.addInputServicePipeline(ctx, &pipelines[i],
			b.addOTLPReceiver(),
			b.addKymaStatsReceiver(),
			b.addMemoryLimiterProcessor(),
			b.addRoutingConnectorAsExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add input service pipeline: %w", err)
		}

		if err := b.addEnrichmentServicePipeline(ctx, &pipelines[i],
			b.addRoutingConnectorAsReceiver(),
			b.addK8sAttributesProcessor(opts),
			b.addServiceEnrichmentProcessor(),
			b.addForwardConnectorAsExporter(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add enrichment service pipeline: %w", err)
		}

		if err := b.addOutputServicePipeline(ctx, &pipelines[i],
			b.addRoutingConnectorAsReceiver(),
			b.addForwardConnectorAsReceiver(),
			b.addSetInstrumentationScopeProcessor(opts),
			// Input source filters (in service.go order)
			b.addDropIfRuntimeInputDisabledProcessor(),
			b.addDropIfPrometheusInputDisabledProcessor(),
			b.addDropIfIstioInputDisabledProcessor(),
			b.addDropEnvoyMetricsIfDisabledProcessor(),
			b.addDropIfOTLPInputDisabledProcessor(),
			// Runtime resource filters (in service.go order)
			b.addDropRuntimePodMetricsProcessor(),
			b.addDropRuntimeContainerMetricsProcessor(),
			b.addDropRuntimeNodeMetricsProcessor(),
			b.addDropRuntimeVolumeMetricsProcessor(),
			b.addDropRuntimeDeploymentMetricsProcessor(),
			b.addDropRuntimeDaemonSetMetricsProcessor(),
			b.addDropRuntimeStatefulSetMetricsProcessor(),
			b.addDropRuntimeJobMetricsProcessor(),
			// Diagnostic metric filters (in service.go order)
			b.addDropDiagnosticMetricsIfInputSourceIstioProcessor(),
			b.addDropDiagnosticMetricsIfInputSourcePrometheusProcessor(),
		); err != nil {
			return nil, nil, fmt.Errorf("failed to add output service pipeline: %w", err)
		}
	}

	return b.config, b.envVars, nil
}

// Type aliases for common builder patterns
type buildComponentFunc = common.BuildComponentFunc[*telemetryv1alpha1.MetricPipeline]
type componentConfigFunc = common.ComponentConfigFunc[*telemetryv1alpha1.MetricPipeline]
type exporterComponentConfigFunc = common.ExporterComponentConfigFunc[*telemetryv1alpha1.MetricPipeline]
type componentIDFunc = common.ComponentIDFunc[*telemetryv1alpha1.MetricPipeline]

// staticComponentID returns a ComponentIDFunc that always returns the same component ID independent of the MetricPipeline
var staticComponentID = common.StaticComponentID[*telemetryv1alpha1.MetricPipeline]

func (b *Builder) addInputServicePipeline(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatInputMetricServicePipelineID(mp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, mp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addEnrichmentServicePipeline(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatEnrichmentServicePipelineID(mp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, mp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addOutputServicePipeline(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline, fs ...buildComponentFunc) error {
	// Add an empty pipeline to the config
	pipelineID := formatOutputServicePipelineID(mp)
	b.config.Service.Pipelines[pipelineID] = common.Pipeline{}

	for _, f := range fs {
		if err := f(ctx, mp); err != nil {
			return fmt.Errorf("failed to add component: %w", err)
		}
	}

	return nil
}

func (b *Builder) addInputReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(b.config, componentIDFunc, configFunc, formatInputMetricServicePipelineID)
}

func (b *Builder) addInputProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(b.config, componentIDFunc, configFunc, formatInputMetricServicePipelineID)
}

func (b *Builder) addInputExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, formatInputMetricServicePipelineID)
}

func (b *Builder) addEnrichmentReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(b.config, componentIDFunc, configFunc, formatEnrichmentServicePipelineID)
}

func (b *Builder) addEnrichmentProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(b.config, componentIDFunc, configFunc, formatEnrichmentServicePipelineID)
}

func (b *Builder) addEnrichmentExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, formatEnrichmentServicePipelineID)
}

func (b *Builder) addOutputReceiver(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddReceiver(b.config, componentIDFunc, configFunc, formatOutputServicePipelineID)
}

func (b *Builder) addOutputProcessor(componentIDFunc componentIDFunc, configFunc componentConfigFunc) buildComponentFunc {
	return common.AddProcessor(b.config, componentIDFunc, configFunc, formatOutputServicePipelineID)
}

func (b *Builder) addOutputExporter(componentIDFunc componentIDFunc, configFunc exporterComponentConfigFunc) buildComponentFunc {
	return common.AddExporter(b.config, b.envVars, componentIDFunc, configFunc, formatOutputServicePipelineID)
}

func (b *Builder) addOTLPReceiver() buildComponentFunc {
	return b.addInputReceiver(
		staticComponentID(common.ComponentIDOTLPReceiver),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.OTLPReceiver{
				Protocols: common.ReceiverProtocols{
					HTTP: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPHTTP),
					},
					GRPC: common.Endpoint{
						Endpoint: fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, ports.OTLPGRPC),
					},
				},
			}
		},
	)
}

func (b *Builder) addKymaStatsReceiver() buildComponentFunc {
	return b.addInputReceiver(
		staticComponentID(common.ComponentIDKymaStatsReceiver),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &KymaStatsReceiver{
				AuthType:           "serviceAccount",
				K8sLeaderElector:   "k8s_leader_elector",
				CollectionInterval: "30s",
				Resources: []ModuleGVR{
					{Group: "operator.kyma-project.io", Version: "v1alpha1", Resource: "telemetries"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "logpipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "metricpipelines"},
					{Group: "telemetry.kyma-project.io", Version: "v1alpha1", Resource: "tracepipelines"},
				},
			}
		},
	)
}

//nolint:mnd // hardcoded values
func (b *Builder) addMemoryLimiterProcessor() buildComponentFunc {
	return b.addInputProcessor(
		staticComponentID(common.ComponentIDMemoryLimiterProcessor),
		func(lp *telemetryv1alpha1.MetricPipeline) any {
			return &common.MemoryLimiter{
				CheckInterval:        "1s",
				LimitPercentage:      75,
				SpikeLimitPercentage: 15,
			}
		},
	)
}

func (b *Builder) addRoutingConnectorAsExporter() buildComponentFunc {
	return b.addInputExporter(
		staticComponentID(common.ComponentIDRoutingConnector),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return enrichmentRoutingConnectorConfig(mp), nil, nil
		},
	)
}

func (b *Builder) addRoutingConnectorAsReceiver() buildComponentFunc {
	return b.addEnrichmentReceiver(
		staticComponentID(common.ComponentIDRoutingConnector),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return enrichmentRoutingConnectorConfig(mp)
		},
	)
}

func (b *Builder) addK8sAttributesProcessor(opts BuildOptions) buildComponentFunc {
	return b.addEnrichmentProcessor(
		staticComponentID(common.ComponentIDK8sAttributesProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.K8sAttributesProcessorConfig(opts.Enrichments)
		},
	)
}

func (b *Builder) addServiceEnrichmentProcessor() buildComponentFunc {
	return b.addEnrichmentProcessor(
		staticComponentID(common.ComponentIDServiceEnrichmentProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.ResolveServiceNameConfig()
		},
	)
}

func (b *Builder) addForwardConnectorAsExporter() buildComponentFunc {
	return b.addEnrichmentExporter(
		staticComponentID(common.ComponentIDForwardConnector),
		func(ctx context.Context, mp *telemetryv1alpha1.MetricPipeline) (any, common.EnvVars, error) {
			return &common.ForwardConnector{}, nil, nil
		},
	)
}

func (b *Builder) addForwardConnectorAsReceiver() buildComponentFunc {
	return b.addOutputReceiver(
		staticComponentID(common.ComponentIDForwardConnector),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return &common.ForwardConnector{}
		},
	)
}

func (b *Builder) addSetInstrumentationScopeProcessor(opts BuildOptions) buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID(common.ComponentIDSetInstrumentationScopeKymaProcessor),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			return common.InstrumentationScopeProcessorConfig(opts.InstrumentationScopeVersion, common.InputSourceKyma)
		},
	)
}

func (b *Builder) addDropIfRuntimeInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-if-input-source-runtime"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{common.ScopeNameEquals(common.InstrumentationScopeRuntime)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfPrometheusInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-if-input-source-prometheus"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`resource.attributes["kyma.input.name"] == "prometheus"`},
				},
			}
		},
	)
}

func (b *Builder) addDropIfIstioInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-if-input-source-istio"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{common.ScopeNameEquals(common.InstrumentationScopeIstio)},
				},
			}
		},
	)
}

func (b *Builder) addDropIfOTLPInputDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-if-input-source-otlp"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsOTLPInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/otlp"`},
				},
			}
		},
	)
}

func (b *Builder) addDropEnvoyMetricsIfDisabledProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-envoy-metrics-if-disabled"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) && metricpipelineutils.IsEnvoyMetricsEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`IsMatch(name, "^envoy_.*") and instrumentation_scope.name == "io.kyma-project.telemetry/istio"`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimePodMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-pod-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimePodInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.pod\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeContainerMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-container-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeContainerInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.container\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeNodeMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-node-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeNodeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.node\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeVolumeMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-volume-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeVolumeInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.volume\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeDeploymentMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-deployment-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeDeploymentInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.deployment\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeDaemonSetMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-daemonset-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeDaemonSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.daemonset\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeStatefulSetMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-statefulset-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeStatefulSetInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.statefulset\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropRuntimeJobMetricsProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-runtime-job-metrics"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsRuntimeInputEnabled(mp.Spec.Input) && metricpipelineutils.IsRuntimeJobInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/runtime" and IsMatch(name, "^k8s\\.job\\.")`},
				},
			}
		},
	)
}

func (b *Builder) addDropDiagnosticMetricsIfInputSourcePrometheusProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-diagnostic-metrics-if-input-source-prometheus"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsPrometheusInputEnabled(mp.Spec.Input) && metricpipelineutils.IsPrometheusDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/prometheus" and (name == "up" or name == "scrape_duration_seconds" or name == "scrape_samples_scraped" or name == "scrape_samples_post_metric_relabeling" or name == "scrape_series_added")`},
				},
			}
		},
	)
}

func (b *Builder) addDropDiagnosticMetricsIfInputSourceIstioProcessor() buildComponentFunc {
	return b.addOutputProcessor(
		staticComponentID("filter/drop-diagnostic-metrics-if-input-source-istio"),
		func(mp *telemetryv1alpha1.MetricPipeline) any {
			if metricpipelineutils.IsIstioInputEnabled(mp.Spec.Input) && metricpipelineutils.IsIstioDiagnosticInputEnabled(mp.Spec.Input) {
				return nil
			}

			return &FilterProcessor{
				Metrics: FilterProcessorMetrics{
					Metric: []string{`instrumentation_scope.name == "io.kyma-project.telemetry/istio" and (name == "up" or name == "scrape_duration_seconds" or name == "scrape_samples_scraped" or name == "scrape_samples_post_metric_relabeling" or name == "scrape_series")`},
				},
			}
		},
	)
}

func enrichmentRoutingConnectorConfig(mp *telemetryv1alpha1.MetricPipeline) common.RoutingConnector {
	enrichmentPipelineID := formatEnrichmentServicePipelineID(mp)
	outputPipelineID := formatOutputServicePipelineID(mp)

	return common.RoutingConnector{
		DefaultPipelines: []string{enrichmentPipelineID},
		ErrorMode:        "ignore",
		Table: []common.RoutingConnectorTableEntry{
			{
				Statement: fmt.Sprintf("route() where attributes[\"%s\"] == \"true\"", common.SkipEnrichmentAttribute),
				Pipelines: []string{outputPipelineID},
			},
		},
	}
}

func formatInputMetricServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-input", mp.Name)
}

func formatEnrichmentServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-attributes-enrichment", mp.Name)
}

func formatOutputServicePipelineID(mp *telemetryv1alpha1.MetricPipeline) string {
	return fmt.Sprintf("metrics/%s-output", mp.Name)
}

// // addComponents enriches a Config (receivers, processors, exporters etc.) with components for a given telemetryv1alpha1.MetricPipeline.
// func (b *Builder) addComponents(
// 	ctx context.Context,
// 	pipeline *telemetryv1alpha1.MetricPipeline,
// 	queueSize int,
// ) error {
// 	b.addDiagnosticMetricsDropFilters(pipeline)
// 	b.addInputSourceFilters(pipeline)
// 	b.addRuntimeResourcesFilters(pipeline)
// 	b.addNamespaceFilters(pipeline)
// 	b.addUserDefinedTransformProcessor(pipeline)
// 	b.addConnectors(pipeline.Name)

// 	return b.addOTLPExporter(ctx, pipeline, queueSize)
// }

// func (b *Builder) addDiagnosticMetricsDropFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
// 	input := pipeline.Spec.Input

// 	if metricpipelineutils.IsPrometheusInputEnabled(input) && !metricpipelineutils.IsPrometheusDiagnosticInputEnabled(input) {
// 		b.config.Processors.DropDiagnosticMetricsIfInputSourcePrometheus = dropDiagnosticMetricsFilterConfig(inputSourceEquals(common.InputSourcePrometheus))
// 	}

// 	if metricpipelineutils.IsIstioInputEnabled(input) && !metricpipelineutils.IsIstioDiagnosticInputEnabled(input) {
// 		b.config.Processors.DropDiagnosticMetricsIfInputSourceIstio = dropDiagnosticMetricsFilterConfig(inputSourceEquals(common.InputSourceIstio))
// 	}
// }

// func (b *Builder) addInputSourceFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
// 	input := pipeline.Spec.Input

// 	if !metricpipelineutils.IsRuntimeInputEnabled(input) {
// 		b.config.Processors.DropIfInputSourceRuntime = dropIfInputSourceRuntimeProcessorConfig()
// 	}

// 	if !metricpipelineutils.IsPrometheusInputEnabled(input) {
// 		b.config.Processors.DropIfInputSourcePrometheus = dropIfInputSourcePrometheusProcessorConfig()
// 	}

// 	if !metricpipelineutils.IsIstioInputEnabled(input) {
// 		b.config.Processors.DropIfInputSourceIstio = dropIfInputSourceIstioProcessorConfig()
// 	}

// 	if !metricpipelineutils.IsOTLPInputEnabled(input) {
// 		b.config.Processors.DropIfInputSourceOTLP = dropIfInputSourceOTLPProcessorConfig()
// 	}

// 	if !metricpipelineutils.IsIstioInputEnabled(input) || !metricpipelineutils.IsEnvoyMetricsEnabled(input) {
// 		b.config.Processors.DropIfEnvoyMetricsDisabled = dropIfEnvoyMetricsDisabledProcessorConfig()
// 	}
// }

// func (b *Builder) addRuntimeResourcesFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
// 	input := pipeline.Spec.Input

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimePodInputEnabled(input) {
// 		b.config.Processors.DropRuntimePodMetrics = dropRuntimePodMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeContainerInputEnabled(input) {
// 		b.config.Processors.DropRuntimeContainerMetrics = dropRuntimeContainerMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeNodeInputEnabled(input) {
// 		b.config.Processors.DropRuntimeNodeMetrics = dropRuntimeNodeMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeVolumeInputEnabled(input) {
// 		b.config.Processors.DropRuntimeVolumeMetrics = dropRuntimeVolumeMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDeploymentInputEnabled(input) {
// 		b.config.Processors.DropRuntimeDeploymentMetrics = dropRuntimeDeploymentMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeStatefulSetInputEnabled(input) {
// 		b.config.Processors.DropRuntimeStatefulSetMetrics = dropRuntimeStatefulSetMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeDaemonSetInputEnabled(input) {
// 		b.config.Processors.DropRuntimeDaemonSetMetrics = dropRuntimeDaemonSetMetricsProcessorConfig()
// 	}

// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && !metricpipelineutils.IsRuntimeJobInputEnabled(input) {
// 		b.config.Processors.DropRuntimeJobMetrics = dropRuntimeJobMetricsProcessorConfig()
// 	}
// }

// func (b *Builder) addNamespaceFilters(pipeline *telemetryv1alpha1.MetricPipeline) {
// 	if b.config.Processors.Dynamic == nil {
// 		b.config.Processors.Dynamic = make(map[string]any)
// 	}

// 	input := pipeline.Spec.Input
// 	if metricpipelineutils.IsRuntimeInputEnabled(input) && shouldFilterByNamespace(input.Runtime.Namespaces) {
// 		processorID := formatNamespaceFilterID(pipeline.Name, common.InputSourceRuntime)
// 		b.config.Processors.Dynamic[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Runtime.Namespaces, inputSourceEquals(common.InputSourceRuntime))
// 	}

// 	if metricpipelineutils.IsPrometheusInputEnabled(input) && shouldFilterByNamespace(input.Prometheus.Namespaces) {
// 		processorID := formatNamespaceFilterID(pipeline.Name, common.InputSourcePrometheus)
// 		b.config.Processors.Dynamic[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Prometheus.Namespaces, common.ResourceAttributeEquals(common.KymaInputNameAttribute, common.KymaInputPrometheus))
// 	}

// 	if metricpipelineutils.IsIstioInputEnabled(input) && shouldFilterByNamespace(input.Istio.Namespaces) {
// 		processorID := formatNamespaceFilterID(pipeline.Name, common.InputSourceIstio)
// 		b.config.Processors.Dynamic[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.Istio.Namespaces, inputSourceEquals(common.InputSourceIstio))
// 	}

// 	if metricpipelineutils.IsOTLPInputEnabled(input) && input.OTLP != nil && shouldFilterByNamespace(input.OTLP.Namespaces) {
// 		processorID := formatNamespaceFilterID(pipeline.Name, common.InputSourceOTLP)
// 		b.config.Processors.Dynamic[processorID] = filterByNamespaceProcessorConfig(pipeline.Spec.Input.OTLP.Namespaces, otlpInputSource())
// 	}
// }

// func (b *Builder) addUserDefinedTransformProcessor(pipeline *telemetryv1alpha1.MetricPipeline) {
// 	if len(pipeline.Spec.Transforms) == 0 {
// 		return
// 	}

// 	transformStatements := common.TransformSpecsToProcessorStatements(pipeline.Spec.Transforms)
// 	transformProcessor := common.MetricTransformProcessorConfig(transformStatements)

// 	processorID := formatUserDefinedTransformProcessorID(pipeline.Name)
// 	b.config.Processors.Dynamic[processorID] = transformProcessor
// }

// func (b *Builder) addConnectors(pipelineName string) {
// 	forwardConnectorID := formatForwardConnectorID(pipelineName)
// 	b.config.Connectors[forwardConnectorID] = struct{}{}

// 	routingConnectorID := formatRoutingConnectorID(pipelineName)
// 	b.config.Connectors[routingConnectorID] = routingConnectorConfig(pipelineName)
// }

// func (b *Builder) addOTLPExporter(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline, queueSize int) error {
// 	otlpExporterBuilder := common.NewOTLPExporterConfigBuilder(
// 		b.Reader,
// 		pipeline.Spec.Output.OTLP,
// 		pipeline.Name,
// 		queueSize,
// 		common.SignalTypeMetric,
// 	)

// 	otlpExporterConfig, otlpExporterEnvVars, err := otlpExporterBuilder.OTLPExporterConfig(ctx)
// 	if err != nil {
// 		return fmt.Errorf("failed to make otlp exporter config: %w", err)
// 	}

// 	maps.Copy(b.envVars, otlpExporterEnvVars)

// 	exporterID := common.ExporterID(pipeline.Spec.Output.OTLP.Protocol, pipeline.Name)
// 	b.config.Exporters[exporterID] = Exporter{OTLP: otlpExporterConfig}

// 	return nil
// }

// func shouldFilterByNamespace(namespaceSelector *telemetryv1alpha1.NamespaceSelector) bool {
// 	return namespaceSelector != nil && (len(namespaceSelector.Include) > 0 || len(namespaceSelector.Exclude) > 0)
// }
