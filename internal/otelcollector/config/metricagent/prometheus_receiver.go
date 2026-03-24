package metricagent

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

type NodeAffiliatedResource string

const (
	NodeAffiliatedPod      NodeAffiliatedResource = "pod"
	NodeAffiliatedEndpoint NodeAffiliatedResource = "endpoint"
)

type AnnotatedResource string

const (
	AnnotatedPod                   AnnotatedResource = "pod"
	AnnotatedService               AnnotatedResource = "service"
	PodNodeSelectorFieldExpression string            = "spec.nodeName=${MY_NODE_NAME}"
)

const (
	sampleLimit              = 50000
	bodySizeLimit            = "20MB"
	appPodsJobName           = "app-pods"
	appServicesJobName       = "app-services"
	appServicesSecureJobName = "app-services-secure"
)

// prometheusPodsReceiverConfig creates a Prometheus configuration for scraping Pods that are annotated with prometheus.io annotations.
func prometheusPodsReceiverConfig(collectionInterval time.Duration) *PrometheusReceiverConfig {
	var config PrometheusReceiverConfig

	scrapeConfig := Scrape{
		ScrapeInterval:             collectionInterval,
		SampleLimit:                sampleLimit,
		BodySizeLimit:              bodySizeLimit,
		KubernetesDiscoveryConfigs: discoveryConfigWithNodeSelector(RolePod),
		JobName:                    appPodsJobName,
		RelabelConfigs:             prometheusPodsRelabelConfigs(),
	}

	config.Prometheus.ScrapeConfigs = append(config.Prometheus.ScrapeConfigs, scrapeConfig)

	return &config
}

// prometheusServicesReceiverConfig creates a Prometheus configuration for scraping Services that are annotated with prometheus.io annotations.
// If Istio is enabled, an additional scrape job config is generated (suffixed with -secure) to scrape annotated Services over HTTPS using Istio certificate.
// Istio certificate is expected to be mounted at the provided path using the proxy.istio.io/config annotation.
// See more: https://istio.io/latest/docs/ops/integrations/prometheus/#tls-settings
func prometheusServicesReceiverConfig(opts BuildOptions, collectionInterval time.Duration) *PrometheusReceiverConfig {
	var config PrometheusReceiverConfig

	baseScrapeConfig := Scrape{
		ScrapeInterval:             collectionInterval,
		SampleLimit:                sampleLimit,
		KubernetesDiscoveryConfigs: discoveryConfigWithNodeSelector(RoleEndpoints),
	}

	httpScrapeConfig := baseScrapeConfig
	httpScrapeConfig.JobName = appServicesJobName
	httpScrapeConfig.RelabelConfigs = prometheusEndpointsRelabelConfigs(false)
	config.Prometheus.ScrapeConfigs = append(config.Prometheus.ScrapeConfigs, httpScrapeConfig)

	// If Istio is active, generate an additional scrape config for scraping annotated Services over HTTPS
	if opts.IstioActive {
		httpsScrapeConfig := baseScrapeConfig
		httpsScrapeConfig.JobName = appServicesSecureJobName
		httpsScrapeConfig.RelabelConfigs = prometheusEndpointsRelabelConfigs(true)
		httpsScrapeConfig.TLS = tlsConfig(opts.IstioCertPath)
		config.Prometheus.ScrapeConfigs = append(config.Prometheus.ScrapeConfigs, httpsScrapeConfig)
	}

	return &config
}

// prometheusPodsRelabelConfigs generates a set of relabel configs for the Pod role type.
// They restrict Pods that are selected for scraping and set internal labels (__address__, __scheme__, etc.).
// See more: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#pod.
//
// Only Pods without Istio sidecars are selected.
func prometheusPodsRelabelConfigs() []Relabel {
	return []Relabel{
		keepIfRunningOnSameNode(NodeAffiliatedPod),
		keepIfScrapingEnabled(AnnotatedPod),
		dropIfPodNotRunning(),
		dropIfInitContainer(),
		inferSchemeFromIstioInjectedLabel(),
		dropIfSchemeHTTPS(),
		inferMetricsPathFromAnnotation(AnnotatedPod),
		inferAddressFromAnnotation(AnnotatedPod),
		inferURLParamFromAnnotation(AnnotatedPod),
	}
}

// prometheusEndpointsRelabelConfigs generates a set of relabel configs for the Endpoint role type.
// They restrict Service Endpoints that are selected for scraping and set internal labels (__address__, __scheme__, etc.).
// See more: https://prometheus.io/docs/prometheus/latest/configuration/configuration/#endpoint.
//
// If requireHTTPS is true, only Endpoints backed by Pods with Istio sidecars or those explicitly marked with prometheus.io/scheme=http annotations are selected.
// If requireHTTPS is false, only Endpoints backed by Pods wuthout Istio sidecars or those marked with prometheus.io/scheme=https annotation are selcted.
func prometheusEndpointsRelabelConfigs(requireHTTPS bool) []Relabel {
	relabelConfigs := []Relabel{
		keepIfRunningOnSameNode(NodeAffiliatedEndpoint),
		keepIfScrapingEnabled(AnnotatedService),
		dropIfPodNotRunning(),
		dropIfInitContainer(),
		dropIfIstioProxy(),
		inferSchemeFromIstioInjectedLabel(),
		inferSchemeFromAnnotation(AnnotatedService),
		inferURLParamFromAnnotation(AnnotatedService),
	}

	if requireHTTPS {
		relabelConfigs = append(relabelConfigs, dropIfSchemeHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, dropIfSchemeHTTPS())
	}

	return append(relabelConfigs,
		inferMetricsPathFromAnnotation(AnnotatedService),
		inferAddressFromAnnotation(AnnotatedService),
		inferServiceFromMetaLabel())
}

func tlsConfig(istioCertPath string) *TLS {
	istioCAFile := filepath.Join(istioCertPath, "root-cert.pem")
	istioCertFile := filepath.Join(istioCertPath, "cert-chain.pem")
	istioKeyFile := filepath.Join(istioCertPath, "key.pem")

	return &TLS{
		CAFile:             istioCAFile,
		CertFile:           istioCertFile,
		KeyFile:            istioKeyFile,
		InsecureSkipVerify: true,
	}
}

func prometheusIstioReceiverConfig(envoyMetricsEnabled bool, collectionInterval time.Duration) *PrometheusReceiverConfig {
	metricNames := "istio_.*"
	if envoyMetricsEnabled {
		metricNames = strings.Join([]string{"envoy_.*", metricNames}, "|")
	}

	return &PrometheusReceiverConfig{
		Prometheus: PrometheusScrape{
			ScrapeConfigs: []Scrape{
				{
					JobName:                    "istio-proxy",
					SampleLimit:                sampleLimit,
					MetricsPath:                "/stats/prometheus",
					ScrapeInterval:             collectionInterval,
					KubernetesDiscoveryConfigs: discoveryConfigWithNodeSelector(RolePod),
					RelabelConfigs: []Relabel{
						keepIfRunningOnSameNode(NodeAffiliatedPod),
						keepIfIstioProxy(),
						keepIfContainerWithEnvoyPort(),
						dropIfPodNotRunning(),
					},
					MetricRelabelConfigs: []Relabel{
						{
							SourceLabels: []string{"__name__"},
							Regex:        metricNames,
							Action:       Keep,
						},
					},
				},
			},
		},
	}
}

func keepIfRunningOnSameNode(nodeAffiliated NodeAffiliatedResource) Relabel {
	return Relabel{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_node_name", nodeAffiliated)},
		Regex:        fmt.Sprintf("${%s}", common.EnvVarCurrentNodeName),
		Action:       Keep,
	}
}

func keepIfScrapingEnabled(annotated AnnotatedResource) Relabel {
	return Relabel{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scrape", annotated)},
		Regex:        "true",
		Action:       Keep,
	}
}

func keepIfIstioProxy() Relabel {
	return Relabel{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       Keep,
		Regex:        "istio-proxy",
	}
}

func keepIfContainerWithEnvoyPort() Relabel {
	return Relabel{
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
func inferSchemeFromIstioInjectedLabel() Relabel {
	return Relabel{
		SourceLabels: []string{"__meta_kubernetes_pod_label_security_istio_io_tlsMode"},
		Action:       Replace,
		TargetLabel:  "__scheme__",
		Regex:        "(istio)",
		Replacement:  "https",
	}
}

func inferSchemeFromAnnotation(annotated AnnotatedResource) Relabel {
	return Relabel{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_scheme", annotated)},
		Action:       Replace,
		Regex:        "(https?)",
		TargetLabel:  "__scheme__",
	}
}

func inferMetricsPathFromAnnotation(annotated AnnotatedResource) Relabel {
	return Relabel{
		SourceLabels: []string{fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_path", annotated)},
		Action:       Replace,
		Regex:        "(.+)",
		TargetLabel:  "__metrics_path__",
	}
}

func inferAddressFromAnnotation(annotated AnnotatedResource) Relabel {
	return Relabel{
		SourceLabels: []string{"__address__", fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_port", annotated)},
		Action:       Replace,
		Regex:        "([^:]+)(?::\\d+)?;(\\d+)",
		Replacement:  "$$1:$$2",
		TargetLabel:  "__address__",
	}
}

func inferServiceFromMetaLabel() Relabel {
	return Relabel{
		SourceLabels: []string{"__meta_kubernetes_service_name"},
		Action:       Replace,
		TargetLabel:  "service",
	}
}

func dropIfPodNotRunning() Relabel {
	return Relabel{
		SourceLabels: []string{"__meta_kubernetes_pod_phase"},
		Action:       Drop,
		Regex:        "Pending|Succeeded|Failed",
	}
}

func dropIfInitContainer() Relabel {
	return Relabel{
		SourceLabels: []string{"__meta_kubernetes_pod_container_init"},
		Action:       Drop,
		Regex:        "(true)",
	}
}

func dropIfIstioProxy() Relabel {
	return Relabel{
		SourceLabels: []string{"__meta_kubernetes_pod_container_name"},
		Action:       Drop,
		Regex:        "(istio-proxy)",
	}
}

func dropIfSchemeHTTP() Relabel {
	return Relabel{
		SourceLabels: []string{"__scheme__"},
		Action:       Drop,
		Regex:        "(http)",
	}
}

func dropIfSchemeHTTPS() Relabel {
	return Relabel{
		SourceLabels: []string{"__scheme__"},
		Action:       Drop,
		Regex:        "(https)",
	}
}

// inferURLParamFromAnnotation extracts and configures the URL parameter
// for scraping based on annotations of the form prometheus.io/param_{name}: {value}.
func inferURLParamFromAnnotation(annotated AnnotatedResource) Relabel {
	return Relabel{
		Regex:       fmt.Sprintf("__meta_kubernetes_%s_annotation_prometheus_io_param_(.+)", annotated),
		Action:      LabelMap,
		Replacement: "__param_$1",
	}
}

func discoveryConfigWithNodeSelector(role Role) []KubernetesDiscovery {
	return []KubernetesDiscovery{
		{
			Role: role,
			Selectors: []K8SDiscoverySelector{
				{
					Role:  RolePod,
					Field: PodNodeSelectorFieldExpression,
				},
			},
		},
	}
}
