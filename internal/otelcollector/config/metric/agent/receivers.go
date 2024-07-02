package agent

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

const scrapeInterval = 30 * time.Second
const sampleLimit = 50000

func makeReceiversConfig(inputs inputSources, runtime runtimeResources, opts BuildOptions) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusAppPods = makePrometheusConfigForPods(opts)
		receiversConfig.PrometheusAppServices = makePrometheusConfigForServices(opts)
	}

	if inputs.runtime {
		receiversConfig.KubeletStats = makeKubeletStatsConfig(runtime)
	}

	if inputs.istio {
		receiversConfig.PrometheusIstio = makePrometheusIstioConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig(runtime runtimeResources) *KubeletStatsReceiver {
	const collectionInterval = "30s"
	const portKubelet = 10250
	var metricGroups []MetricGroupType
	if runtime.container {
		metricGroups = append(metricGroups, MetricGroupTypeContainer)
	}
	if runtime.pod {
		metricGroups = append(metricGroups, MetricGroupTypePod)
	}
	return &KubeletStatsReceiver{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${env:%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       metricGroups,
		Metrics: KubeletMetricsConfig{
			ContainerCPUUsage:       KubeletMetricConfig{Enabled: true},
			ContainerCPUUtilization: KubeletMetricConfig{Enabled: false},
			K8sNodeCPUUsage:         KubeletMetricConfig{Enabled: true},
			K8sNodeCPUUtilization:   KubeletMetricConfig{Enabled: false},
			K8sPodCPUUsage:          KubeletMetricConfig{Enabled: true},
			K8sPodCPUUtilization:    KubeletMetricConfig{Enabled: false},
		},
	}
}

func makePrometheusConfigForPods(opts BuildOptions) *PrometheusReceiver {
	return makePrometheusConfig(opts, "app-pods", RolePod, makePrometheusPodsRelabelConfigs)
}

func makePrometheusConfigForServices(opts BuildOptions) *PrometheusReceiver {
	return makePrometheusConfig(opts, "app-services", RoleEndpoints, makePrometheusServicesRelabelConfigs)
}

func makePrometheusConfig(opts BuildOptions, jobNamePrefix string, role Role, relabelConfigFn func(keepSecure bool) []RelabelConfig) *PrometheusReceiver {
	var config PrometheusReceiver

	baseScrapeConfig := ScrapeConfig{
		ScrapeInterval:             scrapeInterval,
		SampleLimit:                sampleLimit,
		KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: role}},
	}

	httpScrapeConfig := baseScrapeConfig
	httpScrapeConfig.JobName = jobNamePrefix
	httpScrapeConfig.RelabelConfigs = relabelConfigFn(false)
	config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpScrapeConfig)

	if opts.IstioEnabled {
		httpsScrapeConfig := baseScrapeConfig
		httpsScrapeConfig.JobName = jobNamePrefix + "-secure"
		httpsScrapeConfig.RelabelConfigs = relabelConfigFn(true)
		httpsScrapeConfig.TLSConfig = makeTLSConfig(opts.IstioCertPath)
		config.Config.ScrapeConfigs = append(config.Config.ScrapeConfigs, httpsScrapeConfig)
	}

	return &config
}

func makePrometheusPodsRelabelConfigs(keepSecure bool) []RelabelConfig {
	relabelConfigs := []RelabelConfig{
		keepIfRunningOnSameNode(NodeAffiliatedPod),
		keepIfScrapingEnabled(AnnotatedPod),
		dropIfPodNotRunning(),
		dropIfInitContainer(),
		dropIfIstioProxy(),
		inferSchemeFromIstioInjectedLabel(),
		inferSchemeFromAnnotation(AnnotatedPod),
	}

	if keepSecure {
		relabelConfigs = append(relabelConfigs, dropIfSchemeHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, dropIfSchemeHTTPS())
	}

	return append(relabelConfigs,
		inferMetricsPathFromAnnotation(AnnotatedPod),
		inferAddressFromAnnotation(AnnotatedPod))
}

func makePrometheusServicesRelabelConfigs(keepSecure bool) []RelabelConfig {
	relabelConfigs := []RelabelConfig{
		keepIfRunningOnSameNode(NodeAffiliatedEndpoint),
		keepIfScrapingEnabled(AnnotatedService),
		dropIfPodNotRunning(),
		dropIfInitContainer(),
		dropIfIstioProxy(),
		inferSchemeFromIstioInjectedLabel(),
		inferSchemeFromAnnotation(AnnotatedService),
	}

	if keepSecure {
		relabelConfigs = append(relabelConfigs, dropIfSchemeHTTP())
	} else {
		relabelConfigs = append(relabelConfigs, dropIfSchemeHTTPS())
	}

	return append(relabelConfigs,
		inferMetricsPathFromAnnotation(AnnotatedService),
		inferAddressFromAnnotation(AnnotatedService),
		inferServiceFromMetaLabel())
}

func makeTLSConfig(istioCertPath string) *TLSConfig {
	istioCAFile := filepath.Join(istioCertPath, "root-cert.pem")
	istioCertFile := filepath.Join(istioCertPath, "cert-chain.pem")
	istioKeyFile := filepath.Join(istioCertPath, "key.pem")

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
					SampleLimit:                sampleLimit,
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
