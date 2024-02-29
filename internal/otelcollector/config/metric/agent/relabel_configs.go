package agent

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/prometheus"
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

func keepIfRunningOnSameNode(nodeAffiliated NodeAffiliatedResource) prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_node_name", nodeAffiliated)},
		Regex:        fmt.Sprintf("$%s", config.EnvVarCurrentNodeName),
		Action:       prometheus.Keep,
	}
}

func keepIfScrapingEnabled(annotated AnnotatedResource) prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scrape", annotated)},
		Regex:        "true",
		Action:       prometheus.Keep,
	}
}

func keepIfIstioProxy() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       prometheus.Keep,
		Regex:        "istio-proxy",
	}
}

func keepIfContainerWithEnvoyPort() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_port_name"},
		Action:       prometheus.Keep,
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
func inferSchemeFromIstioInjectedLabel() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_label_security_istio_io_tlsMode"},
		Action:       prometheus.Replace,
		TargetLabel:  "__scheme__",
		Regex:        "(istio)",
		Replacement:  "https",
	}
}

func inferSchemeFromAnnotation(annotated AnnotatedResource) prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scheme", annotated)},
		Action:       prometheus.Replace,
		Regex:        "(https?)",
		TargetLabel:  "__scheme__",
	}
}

func inferMetricsPathFromAnnotation(annotated AnnotatedResource) prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_path", annotated)},
		Action:       prometheus.Replace,
		Regex:        "(.+)",
		TargetLabel:  "__metrics_path__",
	}
}

func inferAddressFromAnnotation(annotated AnnotatedResource) prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__address__", fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_port", annotated)},
		Action:       prometheus.Replace,
		Regex:        "([^:]+)(?::\\d+)?;(\\d+)",
		Replacement:  "$$1:$$2",
		TargetLabel:  "__address__",
	}
}

func inferServiceFromMetaLabel() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_service_name"},
		Action:       prometheus.Replace,
		TargetLabel:  "service",
	}
}

func dropIfPodNotRunning() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_phase"},
		Action:       prometheus.Drop,
		Regex:        "Pending|Succeeded|Failed",
	}
}

func dropIfInitContainer() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_init"},
		Action:       prometheus.Drop,
		Regex:        "(true)",
	}
}

func dropIfIstioProxy() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       prometheus.Drop,
		Regex:        "(istio-proxy)",
	}
}

func dropIfSchemeHTTP() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__scheme__"},
		Action:       prometheus.Drop,
		Regex:        "(http)",
	}
}

func dropIfSchemeHTTPS() prometheus.RelabelConfig {
	return prometheus.RelabelConfig{
		SourceLabels: []string{"__scheme__"},
		Action:       prometheus.Drop,
		Regex:        "(https)",
	}
}
