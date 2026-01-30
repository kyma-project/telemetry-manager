package common

const (
	AnnotationKeyChecksumConfig = "checksum/config"

	AnnotationKeyIstioIncludeInboundPorts      = "traffic.sidecar.istio.io/includeInboundPorts"
	AnnotationKeyIstioExcludeInboundPorts      = "traffic.sidecar.istio.io/excludeInboundPorts"
	AnnotationKeyIstioIncludeOutboundPorts     = "traffic.sidecar.istio.io/includeOutboundPorts"
	AnnotationKeyIstioIncludeOutboundIPRanges  = "traffic.sidecar.istio.io/includeOutboundIPRanges"
	AnnotationKeyIstioUserVolumeMount          = "sidecar.istio.io/userVolumeMount"
	AnnotationKeyIstioInterceptionMode         = "sidecar.istio.io/interceptionMode"
	AnnotationValueIstioInterceptionModeTProxy = "TPROXY"
	AnnotationKeyIstioProxyConfig              = "proxy.istio.io/config"

	AnnotationKeyPrometheusScrape = "prometheus.io/scrape"
	AnnotationKeyPrometheusPort   = "prometheus.io/port"
	AnnotationKeyPrometheusScheme = "prometheus.io/scheme"
	AnnotationKeyPrometheusPath   = "prometheus.io/path"

	// AnnotationKeyTelemetryServiceEnrichment is used to specify the service enrichment strategy for telemetry data.
	// It's a temporary annotation to transition from the legacy Kyma enrichment to OpenTelemetry enrichment.
	// For more details see: https://github.com/kyma-project/telemetry-manager/issues/2890
	AnnotationKeyTelemetryServiceEnrichment             = "telemetry.kyma-project.io/service-enrichment"
	AnnotationValueTelemetryServiceEnrichmentOtel       = "otel"
	AnnotationValueTelemetryServiceEnrichmentKymaLegacy = "kyma-legacy"
	AnnotationValueTelemetryServiceEnrichmentDefault    = AnnotationValueTelemetryServiceEnrichmentKymaLegacy
)
