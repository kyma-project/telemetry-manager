package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type NodeAffiliatedResource string

const (
	NodeAffiliatedPod      NodeAffiliatedResource = "pod"
	NodeAffiliatedEndpoint NodeAffiliatedResource = "endpoint"
)

type AnnotatedResource string

const (
	AnnotatedPod     AnnotatedResource = "pod"
	AnnotatedService AnnotatedResource = "service"
)

func keepIfRunningOnSameNode(nodeAffiliated NodeAffiliatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_node_name", nodeAffiliated)},
		Regex:        fmt.Sprintf("$%s", config.EnvVarCurrentNodeName),
		Action:       Keep,
	}
}

func keepIfScrapingEnabled(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scrape", annotated)},
		Regex:        "true",
		Action:       Keep,
	}
}

func keepIfIstioProxy() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       Keep,
		Regex:        "istio-proxy",
	}
}

func keepIfContainerWithEnvoyPort() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_port_name"},
		Action:       Keep,
		Regex:        "http-envoy-prom",
	}
}

// InferSchemeFromIstioInjectedLabel configures the default scraping scheme to HTTPS
// based on the presence of the security.istio.io/tlsMode label in a Pod. This label
// is automatically added by Istio's MutatingWebhook when a sidecar is injected.
//
// When a sidecar is detected (i.e., the label is present), this function sets the scraping scheme to HTTPS.
//
// Note: The HTTPS scheme can be manually overridden by setting the "prometheus.io/scheme"
// annotation on the Pod or the Service.
func inferSchemeFromIstioInjectedLabel() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_label_security_istio_io_tlsMode"},
		Action:       Replace,
		TargetLabel:  "__scheme__",
		Regex:        "(istio)",
		Replacement:  "https",
	}
}

func inferSchemeFromAnnotation(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scheme", annotated)},
		Action:       Replace,
		Regex:        "(https?)",
		TargetLabel:  "__scheme__",
	}
}

func inferMetricsPathFromAnnotation(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_path", annotated)},
		Action:       Replace,
		Regex:        "(.+)",
		TargetLabel:  "__metrics_path__",
	}
}

func inferAddressFromAnnotation(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__address__", fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_port", annotated)},
		Action:       Replace,
		Regex:        "([^:]+)(?::\\d+)?;(\\d+)",
		Replacement:  "$$1:$$2",
		TargetLabel:  "__address__",
	}
}

func inferServiceFromMetaLabel() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_service_name"},
		Action:       Replace,
		TargetLabel:  "service",
	}
}

func dropIfPodNotRunning() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_phase"},
		Action:       Drop,
		Regex:        "Pending|Succeeded|Failed",
	}
}

func dropIfInitContainer() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_init"},
		Action:       Drop,
		Regex:        "(true)",
	}
}

func dropIfIstioProxy() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       Drop,
		Regex:        "(istio-proxy)",
	}
}

func dropIfSchemeHTTP() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__scheme__"},
		Action:       Drop,
		Regex:        "(http)",
	}
}

func dropIfSchemeHTTPS() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__scheme__"},
		Action:       Drop,
		Regex:        "(https)",
	}
}
