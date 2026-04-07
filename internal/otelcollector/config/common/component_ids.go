package common

type ComponentID = string

const (
	// ================================================================================
	// RECEIVERS
	// ================================================================================

	ComponentIDOTLPReceiver                  = "otlp"
	ComponentIDFileLogReceiver               = "filelog/%s" // dynamically filled with pipeline name
	ComponentIDKymaStatsReceiver             = "kymastats"
	ComponentIDK8sClusterReceiver            = "k8s_cluster"
	ComponentIDKubeletStatsReceiver          = "kubeletstats"
	ComponentIDPrometheusAppPodsReceiver     = "prometheus/app-pods"
	ComponentIDPrometheusAppServicesReceiver = "prometheus/app-services"
	ComponentIDPrometheusIstioReceiver       = "prometheus/istio"

	// ================================================================================
	// PROCESSORS
	// ================================================================================

	// Common Processors

	ComponentIDBatchProcessor                          = "batch"
	ComponentIDMemoryLimiterProcessor                  = "memory_limiter"
	ComponentIDK8sAttributesProcessor                  = "k8s_attributes"
	ComponentIDServiceEnrichmentProcessor              = "service_enrichment"
	ComponentIDIstioNoiseFilterProcessor               = "istio_noise_filter"
	ComponentIDUserDefinedFilterProcessor              = "filter/%s-user-defined-%s"    // dynamically filled with pipeline type prefix and pipeline name suffix (example: filter/tracepipeline-user-defined-mypipeline)
	ComponentIDUserDefinedTransformProcessor           = "transform/%s-user-defined-%s" // dynamically filled with pipeline type prefix and pipeline name suffix (example: transform/tracepipeline-user-defined-mypipeline)
	ComponentIDSetInstrumentationScopeKymaProcessor    = "transform/set-instrumentation-scope-kyma"
	ComponentIDSetInstrumentationScopeRuntimeProcessor = "transform/set-instrumentation-scope-runtime"
	ComponentIDInsertClusterAttributesProcessor        = "transform/insert-cluster-attributes"
	ComponentIDDropKymaAttributesProcessor             = "transform/drop-kyma-attributes"
	ComponentIDDropUnknownServiceNameProcessor         = "transform/drop-unknown-service-name"

	ComponentIDSetKymaInputNameRuntimeProcessor    ComponentID = "transform/set-kyma-input-name-runtime"
	ComponentIDSetKymaInputNameIstioProcessor      ComponentID = "transform/set-kyma-input-name-istio"
	ComponentIDSetKymaInputNamePrometheusProcessor ComponentID = "transform/set-kyma-input-name-prometheus"
	ComponentIDSetKymaInputNameKymaProcessor       ComponentID = "transform/set-kyma-input-name-kyma"
	ComponentIDSetKymaInputNameOTLPProcessor       ComponentID = "transform/set-kyma-input-name-otlp"

	// Log-Specific Processors

	ComponentIDNamespaceFilterProcessor       = "filter/%s-filter-by-namespace" // dynamically filled with log pipeline name (example: filter/mylogpipeline-filter-by-namespace)
	ComponentIDSetObservedTimeIfZeroProcessor = "transform/set-observed-time-if-zero"
	ComponentIDIstioEnrichmentProcessor       = "istio_enrichment"

	// Metric-Specific Processors

	ComponentIDDropIfInputSourceRuntimeProcessor           = "filter/drop-if-input-source-runtime"
	ComponentIDDropIfInputSourcePrometheusProcessor        = "filter/drop-if-input-source-prometheus"
	ComponentIDDropIfInputSourceIstioProcessor             = "filter/drop-if-input-source-istio"
	ComponentIDDropIfInputSourceOTLPProcessor              = "filter/drop-if-input-source-otlp"
	ComponentIDDropEnvoyMetricsIfDisabledProcessor         = "filter/drop-envoy-metrics-if-disabled"
	ComponentIDNamespacePerInputFilterProcessor            = "filter/%s-filter-by-namespace-%s-input" // dynamically filled with metric pipeline name and input source (example: filter/mymetricpipeline-filter-by-namespace-otlp-input)
	ComponentIDDropRuntimePodMetricsProcessor              = "filter/drop-runtime-pod-metrics"
	ComponentIDDropRuntimeContainerMetricsProcessor        = "filter/drop-runtime-container-metrics"
	ComponentIDDropRuntimeNodeMetricsProcessor             = "filter/drop-runtime-node-metrics"
	ComponentIDDropRuntimeVolumeMetricsProcessor           = "filter/drop-runtime-volume-metrics"
	ComponentIDDropRuntimeDeploymentMetricsProcessor       = "filter/drop-runtime-deployment-metrics"
	ComponentIDDropRuntimeDaemonSetMetricsProcessor        = "filter/drop-runtime-daemonset-metrics"
	ComponentIDDropRuntimeStatefulSetMetricsProcessor      = "filter/drop-runtime-statefulset-metrics"
	ComponentIDDropRuntimeJobMetricsProcessor              = "filter/drop-runtime-job-metrics"
	ComponentIDDropPrometheusDiagnosticMetricsProcessor    = "filter/drop-diagnostic-metrics-if-input-source-prometheus"
	ComponentIDDropIstioDiagnosticMetricsProcessor         = "filter/drop-diagnostic-metrics-if-input-source-istio"
	ComponentIDFilterDropNonPVCVolumesMetricsProcessor     = "filter/drop-non-pvc-volumes-metrics"
	ComponentIDFilterDropVirtualNetworkInterfacesProcessor = "filter/drop-virtual-network-interfaces"
	ComponentIDDropServiceNameProcessor                    = "transform/drop-service-name"
	ComponentIDDropSkipEnrichmentAttributeProcessor        = "transform/drop-skip-enrichment-attribute"
	ComponentIDSetInstrumentationScopePrometheusProcessor  = "transform/set-instrumentation-scope-prometheus"
	ComponentIDSetInstrumentationScopeIstioProcessor       = "transform/set-instrumentation-scope-istio"
	ComponentIDInsertSkipEnrichmentAttributeProcessor      = "transform/insert-skip-enrichment-attribute"

	// Trace-Specific Processors

	ComponentIDDropIstioServiceEnrichmentProcessor = "transform/drop-istio-service-enrichment"

	// ================================================================================
	// EXPORTERS
	// ================================================================================

	ComponentIDOTLPHTTPExporter = "otlp_http/%s" // dynamically filled with pipeline type and name (example: otlp_http/metricpipeline-mypipeline)
	ComponentIDOTLPGRPCExporter = "otlp_grpc/%s" // dynamically filled with pipeline type and name (example: otlp_grpc/metricpipeline-mypipeline)
	ComponentIDOTLPExporter     = "otlp"         // static OTLP exporter

	// ================================================================================
	// CONNECTORS
	// ================================================================================

	ComponentIDEnrichmentConnector             = "forward/enrichment"
	ComponentIDInputConnector                  = "forward/input"
	ComponentIDEnrichmentRoutingConnector      = "routing/enrichment"
	ComponentIDRuntimeInputRoutingConnector    = "routing/runtime-input"
	ComponentIDPrometheusInputRoutingConnector = "routing/prometheus-input"
	ComponentIDIstioInputRoutingConnector      = "routing/istio-input"

	// ================================================================================
	// EXTENSIONS
	// ================================================================================

	ComponentIDK8sLeaderElectorExtension = "k8s_leader_elector"
	ComponentIDFileStorageExtension      = "file_storage"
	ComponentIDHealthCheckExtension      = "health_check"
	ComponentIDPprofExtension            = "pprof"
	ComponentIDOAuth2Extension           = "oauth2client/%s" // dynamically filled with pipeline type and name (example: oauth2client/logpipeline-mypipeline)
	ComponentIDCGroupRuntimeExtension    = "cgroupruntime"
)
