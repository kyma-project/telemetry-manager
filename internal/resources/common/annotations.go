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
)
