package common

import (
	"fmt"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

type ComponentID = string

// ================================================================================
// RECEIVERS
// ================================================================================

const ComponentIDOTLPReceiver ComponentID = "otlp"
const ComponentIDKymaStatsReceiver ComponentID = "kymastats"
const ComponentIDK8sClusterReceiver ComponentID = "k8s_cluster"
const ComponentIDKubeletStatsReceiver ComponentID = "kubeletstats"
const ComponentIDPrometheusAppPodsReceiver ComponentID = "prometheus/app-pods"
const ComponentIDPrometheusAppServicesReceiver ComponentID = "prometheus/app-services"
const ComponentIDPrometheusIstioReceiver ComponentID = "prometheus/istio"

// ComponentIDFileLogReceiver generates a component ID for the filelog receiver specific to a log pipeline.
// Pipeline name is included in the component ID to keep it unique across pipelines.
//
// Example: filelog/mylogpipeline
func ComponentIDFileLogReceiver(pipelineName string) ComponentID {
	return fmt.Sprintf("filelog/%s", pipelineName)
}

// ================================================================================
// PROCESSORS
// ================================================================================

// COMMON PROCESSORS ==============================================================

const ComponentIDBatchProcessor ComponentID = "batch"
const ComponentIDMemoryLimiterProcessor ComponentID = "memory_limiter"
const ComponentIDK8sAttributesProcessor ComponentID = "k8s_attributes"
const ComponentIDServiceEnrichmentProcessor ComponentID = "service_enrichment"
const ComponentIDIstioNoiseFilterProcessor ComponentID = "istio_noise_filter"
const ComponentIDSetInstrumentationScopeKymaProcessor ComponentID = "transform/set-instrumentation-scope-kyma"
const ComponentIDSetInstrumentationScopeRuntimeProcessor ComponentID = "transform/set-instrumentation-scope-runtime"
const ComponentIDInsertClusterAttributesProcessor ComponentID = "transform/insert-cluster-attributes"
const ComponentIDDropKymaAttributesProcessor ComponentID = "transform/drop-kyma-attributes"
const ComponentIDDropUnknownServiceNameProcessor ComponentID = "transform/drop-unknown-service-name"

const ComponentIDSetKymaInputNameRuntimeProcessor ComponentID = "transform/set-kyma-input-name-runtime"
const ComponentIDSetKymaInputNameIstioProcessor ComponentID = "transform/set-kyma-input-name-istio"
const ComponentIDSetKymaInputNamePrometheusProcessor ComponentID = "transform/set-kyma-input-name-prometheus"
const ComponentIDSetKymaInputNameKymaProcessor ComponentID = "transform/set-kyma-input-name-kyma"
const ComponentIDSetKymaInputNameOTLPProcessor ComponentID = "transform/set-kyma-input-name-otlp"

// ComponentIDUserDefinedFilterProcessor generates a component ID for the user-defined filter processor.
// Pipeline type and name are included in the component ID to keep it unique across pipelines.
//
// Example: filter/tracepipeline-user-defined-mypipeline
func ComponentIDUserDefinedFilterProcessor(pipelineRef PipelineRef) ComponentID {
	return fmt.Sprintf("filter/%s-user-defined-%s", pipelineRef.TypePrefix(), pipelineRef.Name())
}

// ComponentIDUserDefinedTransformProcessor generates a component ID for the user-defined transform processor.
// Pipeline type and name are included in the component ID to keep it unique across pipelines.
//
// Example: transform/tracepipeline-user-defined-mypipeline
func ComponentIDUserDefinedTransformProcessor(pipelineRef PipelineRef) ComponentID {
	return fmt.Sprintf("transform/%s-user-defined-%s", pipelineRef.TypePrefix(), pipelineRef.Name())
}

// LOG-SPECIFIC PROCESSORS ========================================================

// ComponentIDNamespaceFilterProcessor generates a component ID for the namespace filter processor specific to a log pipeline.
// Pipeline name is included in the component ID to keep it unique across pipelines.
//
// Explanation:
// Log pipelines have two possible inputs (runtime and OTLP): runtime namespace filtering is handled directly in the filelog
// receiver via path patterns, so only OTLP input requires a dedicated namespace filter processor.
// This means each log pipeline has at most one such processor, so the input type does not need to be part of the component ID.
//
// Example: filter/mylogpipeline-filter-by-namespace
func ComponentIDNamespaceFilterProcessor(pipelineName string) ComponentID {
	return fmt.Sprintf("filter/%s-filter-by-namespace", pipelineName)
}

const ComponentIDSetObservedTimeIfZeroProcessor ComponentID = "transform/set-observed-time-if-zero"
const ComponentIDIstioEnrichmentProcessor ComponentID = "istio_enrichment"

// METRIC-SPECIFIC PROCESSORS =====================================================

const ComponentIDDropIfInputSourceRuntimeProcessor ComponentID = "filter/drop-if-input-source-runtime"
const ComponentIDDropIfInputSourcePrometheusProcessor ComponentID = "filter/drop-if-input-source-prometheus"
const ComponentIDDropIfInputSourceIstioProcessor ComponentID = "filter/drop-if-input-source-istio"
const ComponentIDDropIfInputSourceOTLPProcessor ComponentID = "filter/drop-if-input-source-otlp"
const ComponentIDDropEnvoyMetricsIfDisabledProcessor ComponentID = "filter/drop-envoy-metrics-if-disabled"

// ComponentIDNamespacePerInputFilterProcessor generates a component ID for the namespace filter processor specific to a metric pipeline.
// Pipeline name and input type are included in the component ID to keep it unique across pipelines and inputs.
//
// Explanation:
// Metric pipelines have four possible inputs (runtime, prometheus, istio, OTLP): each output OTel Collector service pipeline can contain
// multiple namespace filter processors, one per input type.
// This means the component ID needs to include both the pipeline name and the input type to keep it unique.
//
// Example: filter/mypipeline-filter-by-namespace-otlp-input
func ComponentIDNamespacePerInputFilterProcessor(pipelineName string, inputSource InputSourceType) ComponentID {
	return fmt.Sprintf("filter/%s-filter-by-namespace-%s-input", pipelineName, inputSource)
}

const ComponentIDDropRuntimePodMetricsProcessor ComponentID = "filter/drop-runtime-pod-metrics"
const ComponentIDDropRuntimeContainerMetricsProcessor ComponentID = "filter/drop-runtime-container-metrics"
const ComponentIDDropRuntimeNodeMetricsProcessor ComponentID = "filter/drop-runtime-node-metrics"
const ComponentIDDropRuntimeVolumeMetricsProcessor ComponentID = "filter/drop-runtime-volume-metrics"
const ComponentIDDropRuntimeDeploymentMetricsProcessor ComponentID = "filter/drop-runtime-deployment-metrics"
const ComponentIDDropRuntimeDaemonSetMetricsProcessor ComponentID = "filter/drop-runtime-daemonset-metrics"
const ComponentIDDropRuntimeStatefulSetMetricsProcessor ComponentID = "filter/drop-runtime-statefulset-metrics"
const ComponentIDDropRuntimeJobMetricsProcessor ComponentID = "filter/drop-runtime-job-metrics"
const ComponentIDDropPrometheusDiagnosticMetricsProcessor ComponentID = "filter/drop-diagnostic-metrics-if-input-source-prometheus"
const ComponentIDDropIstioDiagnosticMetricsProcessor ComponentID = "filter/drop-diagnostic-metrics-if-input-source-istio"
const ComponentIDFilterDropNonPVCVolumesMetricsProcessor ComponentID = "filter/drop-non-pvc-volumes-metrics"
const ComponentIDFilterDropVirtualNetworkInterfacesProcessor ComponentID = "filter/drop-virtual-network-interfaces"
const ComponentIDDropServiceNameProcessor ComponentID = "transform/drop-service-name"
const ComponentIDDropSkipEnrichmentAttributeProcessor ComponentID = "transform/drop-skip-enrichment-attribute"
const ComponentIDSetInstrumentationScopePrometheusProcessor ComponentID = "transform/set-instrumentation-scope-prometheus"
const ComponentIDSetInstrumentationScopeIstioProcessor ComponentID = "transform/set-instrumentation-scope-istio"
const ComponentIDInsertSkipEnrichmentAttributeProcessor ComponentID = "transform/insert-skip-enrichment-attribute"

// TRACE-SPECIFIC PROCESSORS ======================================================

const ComponentIDDropIstioServiceEnrichmentProcessor ComponentID = "transform/drop-istio-service-enrichment"

// ================================================================================
// EXPORTERS
// ================================================================================

// ComponentIDOTLPExporter generates a component ID for the OTLP gRPC or HTTP exporter based on the protocol.
// Pipeline name is included in the component ID to keep it unique across pipelines.
//
// Example: otlp_grpc/metricpipeline-mypipeline, otlp_http/logpipeline-mypipeline
func ComponentIDOTLPExporter(protocol telemetryv1beta1.OTLPProtocol, pipelineRef PipelineRef) ComponentID {
	if protocol == telemetryv1beta1.OTLPProtocolHTTP {
		return fmt.Sprintf("otlp_http/%s", pipelineRef.qualifiedName())
	}

	return fmt.Sprintf("otlp_grpc/%s", pipelineRef.qualifiedName())
}

// ================================================================================
// CONNECTORS
// ================================================================================

const ComponentIDEnrichmentConnector ComponentID = "forward/enrichment"
const ComponentIDInputConnector ComponentID = "forward/input"
const ComponentIDEnrichmentRoutingConnector ComponentID = "routing/enrichment"
const ComponentIDRuntimeInputRoutingConnector ComponentID = "routing/runtime-input"
const ComponentIDPrometheusInputRoutingConnector ComponentID = "routing/prometheus-input"
const ComponentIDIstioInputRoutingConnector ComponentID = "routing/istio-input"

// ================================================================================
// EXTENSIONS
// ================================================================================

const ComponentIDK8sLeaderElectorExtension ComponentID = "k8s_leader_elector"
const ComponentIDFileStorageExtension ComponentID = "file_storage"
const ComponentIDHealthCheckExtension ComponentID = "health_check"
const ComponentIDPprofExtension ComponentID = "pprof"
const ComponentIDCGroupRuntimeExtension ComponentID = "cgroupruntime"

// ComponentIDOAuth2Extension generates a component ID for the OAuth2 client extension.
// Pipeline name is included in the component ID to keep it unique across pipelines.
//
// Example: oauth2client/logpipeline-mypipeline
func ComponentIDOAuth2Extension(pipelineRef PipelineRef) ComponentID {
	return fmt.Sprintf("oauth2client/%s", pipelineRef.qualifiedName())
}
