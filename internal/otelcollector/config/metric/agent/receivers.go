package agent

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

const scrapeInterval = 30 * time.Second
const IstioCertPath = "/etc/istio-output-certs"

var (
	istioCAFile   = filepath.Join(IstioCertPath, "root-cert.pem")
	istioCertFile = filepath.Join(IstioCertPath, "cert-chain.pem")
	istioKeyFile  = filepath.Join(IstioCertPath, "key.pem")
)

func makeReceiversConfig(inputs inputSources, isIstioActive bool) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusSelf = makePrometheusSelfConfig()
		receiversConfig.PrometheusAppPods = makePrometheusAppPodsConfig(isIstioActive)
		receiversConfig.PrometheusAppServices = makePrometheusAppServicesConfig(isIstioActive)
	}

	if inputs.runtime {
		receiversConfig.KubeletStats = makeKubeletStatsConfig()
	}

	if inputs.istio {
		receiversConfig.PrometheusIstio = makePrometheusIstioConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig() *KubeletStatsReceiver {
	const collectionInterval = "30s"
	const portKubelet = 10250
	return &KubeletStatsReceiver{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		Endpoint:           fmt.Sprintf("https://${env:%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod},
	}
}

func makePrometheusSelfConfig() *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:        "opentelemetry-collector",
					ScrapeInterval: scrapeInterval,
					StaticDiscoveryConfigs: []StaticDiscoveryConfig{
						{
							Targets: []string{fmt.Sprintf("${%s}:%d", config.EnvVarCurrentPodIP, ports.Metrics)},
						},
					},
				},
			},
		},
	}
}

func makePrometheusAppPodsConfig(isIstioActive bool) *PrometheusReceiver {
	var config PrometheusReceiver

	httpScrapeConfig := ScrapeConfig{
		JobName:                    "app-pods",
		ScrapeInterval:             scrapeInterval,
		KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RolePod}},
		RelabelConfigs:             makePrometheusAppPodsRelabelConfigs(false),
	}
	config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpScrapeConfig)

	if isIstioActive {
		httpsScrapeConfig := ScrapeConfig{
			JobName:                    "app-pods-secure",
			ScrapeInterval:             scrapeInterval,
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RolePod}},
			RelabelConfigs:             makePrometheusAppPodsRelabelConfigs(true),
			TLSConfig:                  makeTLSConfig(),
		}
		config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpsScrapeConfig)
	}

	return &config
}

func makePrometheusAppPodsRelabelConfigs(isSecure bool) []RelabelConfig {
	relabelConfigs := []RelabelConfig{
		keepIfRunningOnSameNode(NodeAffiliatedPod),
		keepIfScrapingEnabled(AnnotatedPod),
		dropIfPodNotRunning(),
		dropIfInitContainer(),
		dropIfIstioProxy(),
	}

	if isSecure {
		relabelConfigs = append(relabelConfigs, dropIfSchemeAnnotationHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, dropIfSchemeAnnotationHTTPS(), inferSchemeFromIstioInjectedLabel())
	}

	return append(relabelConfigs,
		inferSchemeFromAnnotation(AnnotatedPod),
		inferMetricsPathFromAnnotation(AnnotatedPod),
		inferAddressFromAnnotation(AnnotatedPod))
}

func makePrometheusAppServicesConfig(isIstioActive bool) *PrometheusReceiver {
	var config PrometheusReceiver

	httpScrapeConfig := ScrapeConfig{
		JobName:                    "app-services",
		ScrapeInterval:             scrapeInterval,
		KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RoleEndpoints}},
		RelabelConfigs:             makePrometheusAppServicesRelabelConfigs(false),
	}
	config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpScrapeConfig)

	if isIstioActive {
		httpsScrapeConfig := ScrapeConfig{
			JobName:                    "app-services-secure",
			ScrapeInterval:             scrapeInterval,
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RoleEndpoints}},
			RelabelConfigs:             makePrometheusAppServicesRelabelConfigs(true),
			TLSConfig:                  makeTLSConfig(),
		}
		config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpsScrapeConfig)
	}

	return &config
}

func makePrometheusAppServicesRelabelConfigs(isSecure bool) []RelabelConfig {
	relabelConfigs := []RelabelConfig{
		keepIfRunningOnSameNode(NodeAffiliatedEndpoint),
		keepIfScrapingEnabled(AnnotatedService),
		dropIfPodNotRunning(),
		dropIfInitContainer(),
		dropIfIstioProxy(),
	}

	if isSecure {
		relabelConfigs = append(relabelConfigs, dropIfSchemeAnnotationHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, dropIfSchemeAnnotationHTTPS(), inferSchemeFromIstioInjectedLabel())
	}

	return append(relabelConfigs,
		inferSchemeFromAnnotation(AnnotatedService),
		inferMetricsPathFromAnnotation(AnnotatedService),
		inferAddressFromAnnotation(AnnotatedService),
		inferServiceFromMetaLabel())
}

func makeTLSConfig() *TLSConfig {
	return &TLSConfig{
		CAFile:             istioCAFile,
		CertFile:           istioCertFile,
		KeyFile:            istioKeyFile,
		InsecureSkipVerify: true,
	}
}

func makePrometheusIstioConfig() *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:                    "istio-proxy",
					MetricsPath:                "/stats/prometheus",
					ScrapeInterval:             scrapeInterval,
					KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RolePod}},
					RelabelConfigs: []RelabelConfig{
						keepIfRunningOnSameNode(NodeAffiliatedPod),
						keepIfIstioProxy(),
						keepIfContainerWithEnvoyPort(),
						dropIfPodNotRunning(),
					},
					MetricRelabelConfigs: []RelabelConfig{
						{
							SourceLabels: []string{"__name__"},
							Regex:        "istio_.*",
							Action:       Keep,
						},
					},
				},
			},
		},
	}
}
