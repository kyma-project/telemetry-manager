package common

import "fmt"

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
// Example: filter/tracepipeline-user-defined-mypipeline
func ComponentIDUserDefinedFilterProcessor(pipelineTypePrefix, pipelineName string) ComponentID {
	return fmt.Sprintf("filter/%s-user-defined-%s", pipelineTypePrefix, pipelineName)
}

// ComponentIDUserDefinedTransformProcessor generates a component ID for the user-defined transform processor.
// Example: transform/tracepipeline-user-defined-mypipeline
func ComponentIDUserDefinedTransformProcessor(pipelineTypePrefix, pipelineName string) ComponentID {
	return fmt.Sprintf("transform/%s-user-defined-%s", pipelineTypePrefix, pipelineName)
}

// LOG-SPECIFIC PROCESSORS ========================================================

// ComponentIDNamespaceFilterProcessor generates a component ID for the namespace filter processor specific to a log pipeline.
// This namespace filter component ID is specific to log pipelines, since we deploy them as single instances of OTel pipelines, each having its own namespace filter processor.
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

// ComponentIDNamespacePerInputFilterProcessor generates a component ID for the OTel namespace filter processor specific to a metric pipeline.
// This namespace filter processor is specific to metric pipelines, since we deploy them as separate instances of OTel pipelines, each having its own namespace filter processor, corresponding to its input type.
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

// ComponentIDOTLPHTTPExporter generates a component ID for the OTLP HTTP exporter.
// Example: otlp_http/metricpipeline-mypipeline
func ComponentIDOTLPHTTPExporter(qualifiedName string) ComponentID {
	return fmt.Sprintf("otlp_http/%s", qualifiedName)
}

// ComponentIDOTLPGRPCExporter generates a component ID for the OTLP gRPC exporter.
// Example: otlp_grpc/metricpipeline-mypipeline
func ComponentIDOTLPGRPCExporter(qualifiedName string) ComponentID {
	return fmt.Sprintf("otlp_grpc/%s", qualifiedName)
}

const ComponentIDOTLPExporter ComponentID = "otlp"

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
// Example: oauth2client/logpipeline-mypipeline
func ComponentIDOAuth2Extension(qualifiedName string) ComponentID {
	return fmt.Sprintf("oauth2client/%s", qualifiedName)
}
