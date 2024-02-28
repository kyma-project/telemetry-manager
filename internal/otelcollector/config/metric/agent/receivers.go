package agent

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/prometheus"
)

const scrapeInterval = 30 * time.Second
const IstioCertPath = "/etc/istio-output-certs"
const sampleLimit = 50000

var (
	istioCAFile   = filepath.Join(IstioCertPath, "root-cert.pem")
	istioCertFile = filepath.Join(IstioCertPath, "cert-chain.pem")
	istioKeyFile  = filepath.Join(IstioCertPath, "key.pem")
)

func makeReceiversConfig(inputs inputSources, isIstioActive bool) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusAppPods = makePrometheusConfigForPods(isIstioActive)
		receiversConfig.PrometheusAppServices = makePrometheusConfigForServices(isIstioActive)
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
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${env:%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod},
	}
}

func makePrometheusConfigForPods(isIstioActive bool) *PrometheusReceiver {
	return makePrometheusConfig(isIstioActive, "app-pods", prometheus.RolePod, makePrometheusPodsRelabelConfigs)
}

func makePrometheusConfigForServices(isIstioActive bool) *PrometheusReceiver {
	return makePrometheusConfig(isIstioActive, "app-services", prometheus.RoleEndpoints, makePrometheusServicesRelabelConfigs)
}

func makePrometheusConfig(isIstioActive bool, jobNamePrefix string, role prometheus.Role, relabelConfigFn func(keepSecure bool) []prometheus.RelabelConfig) *PrometheusReceiver {
	var config PrometheusReceiver

	baseScrapeConfig := prometheus.ScrapeConfig{
		ScrapeInterval:             scrapeInterval,
		SampleLimit:                sampleLimit,
		KubernetesDiscoveryConfigs: []prometheus.KubernetesDiscoveryConfig{{Role: role}},
	}

	httpScrapeConfig := baseScrapeConfig
	httpScrapeConfig.JobName = jobNamePrefix
	httpScrapeConfig.RelabelConfigs = relabelConfigFn(false)
	config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpScrapeConfig)

	if isIstioActive {
		httpsScrapeConfig := baseScrapeConfig
		httpsScrapeConfig.JobName = jobNamePrefix + "-secure"
		httpsScrapeConfig.RelabelConfigs = relabelConfigFn(true)
		httpsScrapeConfig.TLSConfig = makeTLSConfig()
		config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpsScrapeConfig)
	}

	return &config
}

func makePrometheusPodsRelabelConfigs(keepSecure bool) []prometheus.RelabelConfig {
	relabelConfigs := []prometheus.RelabelConfig{
		KeepIfRunningOnSameNode(NodeAffiliatedPod),
		KeepIfScrapingEnabled(AnnotatedPod),
		DropIfPodNotRunning(),
		DropIfInitContainer(),
		DropIfIstioProxy(),
		InferSchemeFromIstioInjectedLabel(),
		InferSchemeFromAnnotation(AnnotatedPod),
	}

	if keepSecure {
		relabelConfigs = append(relabelConfigs, DropIfSchemeHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, DropIfSchemeHTTPS())
	}

	return append(relabelConfigs,
		InferMetricsPathFromAnnotation(AnnotatedPod),
		InferAddressFromAnnotation(AnnotatedPod))
}

func makePrometheusServicesRelabelConfigs(keepSecure bool) []prometheus.RelabelConfig {
	relabelConfigs := []prometheus.RelabelConfig{
		KeepIfRunningOnSameNode(NodeAffiliatedEndpoint),
		KeepIfScrapingEnabled(AnnotatedService),
		DropIfPodNotRunning(),
		DropIfInitContainer(),
		DropIfIstioProxy(),
		InferSchemeFromIstioInjectedLabel(),
		InferSchemeFromAnnotation(AnnotatedService),
	}

	if keepSecure {
		relabelConfigs = append(relabelConfigs, DropIfSchemeHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, DropIfSchemeHTTPS())
	}

	return append(relabelConfigs,
		InferMetricsPathFromAnnotation(AnnotatedService),
		InferAddressFromAnnotation(AnnotatedService),
		InferServiceFromMetaLabel())
}

func makeTLSConfig() *prometheus.TLSConfig {
	return &prometheus.TLSConfig{
		CAFile:             istioCAFile,
		CertFile:           istioCertFile,
		KeyFile:            istioKeyFile,
		InsecureSkipVerify: true,
	}
}

func makePrometheusIstioConfig() *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []prometheus.ScrapeConfig{
				{
					JobName:                    "istio-proxy",
					SampleLimit:                sampleLimit,
					MetricsPath:                "/stats/prometheus",
					ScrapeInterval:             scrapeInterval,
					KubernetesDiscoveryConfigs: []prometheus.KubernetesDiscoveryConfig{{Role: prometheus.RolePod}},
					RelabelConfigs: []prometheus.RelabelConfig{
						KeepIfRunningOnSameNode(NodeAffiliatedPod),
						KeepIfIstioProxy(),
						KeepIfContainerWithEnvoyPort(),
						DropIfPodNotRunning(),
					},
					MetricRelabelConfigs: []prometheus.RelabelConfig{
						{
							SourceLabels: []string{"__name__"},
							Regex:        "istio_.*",
							Action:       prometheus.Keep,
						},
					},
				},
			},
		},
	}
}
