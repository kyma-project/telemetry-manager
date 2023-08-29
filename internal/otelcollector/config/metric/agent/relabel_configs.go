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

func keepRunningOnSameNode(nodeAffiliated NodeAffiliatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_node_name", nodeAffiliated)},
		Regex:        fmt.Sprintf("$%s", config.EnvVarCurrentNodeName),
		Action:       Keep,
	}
}

func keepAnnotated(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scrape", annotated)},
		Regex:        "true",
		Action:       Keep,
	}
}

func keepIstioProxyContainer() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       Keep,
		Regex:        "istio-proxy",
	}
}

func keepContainerWithEnvoyPort() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_port_name"},
		Action:       Keep,
		Regex:        "http-envoy-prom",
	}
}

func replaceScheme(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scheme", annotated)},
		Action:       Replace,
		Regex:        "(https?)",
		TargetLabel:  "__scheme__",
	}
}

func replaceMetricPath(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_path", annotated)},
		Action:       Replace,
		Regex:        "(.+)",
		TargetLabel:  "__metrics_path__",
	}
}

func replaceAddress(annotated AnnotatedResource) RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__address__", fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_port", annotated)},
		Action:       Replace,
		Regex:        "([^:]+)(?::\\d+)?;(\\d+)",
		Replacement:  "$$1:$$2",
		TargetLabel:  "__address__",
	}
}

func replaceService() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_service_name"},
		Action:       Replace,
		TargetLabel:  "service",
	}
}

func dropNonRunningPods() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_phase"},
		Action:       Drop,
		Regex:        "Pending|Succeeded|Failed",
	}
}

func dropInitContainers() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_init"},
		Action:       Drop,
		Regex:        "(true)",
	}
}

func dropIstioProxyContainer() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       Drop,
		Regex:        "(istio-proxy)",
	}
}

func replaceSchemeIfSidecarFound() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_pod_label_security_istio_io_tlsMode"},
		Action:       Replace,
		TargetLabel:  "__scheme__",
		Regex:        "(istio)",
		Replacement:  "https",
	}
}

func dropHTTP() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scheme"},
		Action:       Drop,
		Regex:        "(http)",
	}
}

func dropHTTPS() RelabelConfig {
	return RelabelConfig{
		SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scheme"},
		Action:       Drop,
		Regex:        "(https)",
	}
}
