package common

const (
	ChecksumConfigAnnotationKey = "checksum/config"

	IstioExcludeInboundPortsAnnotationKey     = "traffic.sidecar.istio.io/excludeInboundPorts"
	IstioIncludeOutboundPortsAnnotationKey    = "traffic.sidecar.istio.io/includeOutboundPorts"
	IstioIncludeOutboundIPRangesAnnotationKey = "traffic.sidecar.istio.io/includeOutboundIPRanges"
	IstioUserVolumeMountAnnotationKey         = "sidecar.istio.io/userVolumeMount"
	IstioInterceptionModeAnnotationKey        = "sidecar.istio.io/interceptionMode"
	IstioInterceptionModeAnnotationValue      = "TPROXY"
	IstioProxyConfigAnnotationKey             = "proxy.istio.io/config"

	PrometheusScrapeAnnotationKey = "prometheus.io/scrape"
	PrometheusPortAnnotationKey   = "prometheus.io/port"
	PrometheusSchemeAnnotationKey = "prometheus.io/scheme"
	PrometheusPathAnnotationKey   = "prometheus.io/path"
)
