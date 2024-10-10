package agent

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metric"
)

const scrapeInterval = 30 * time.Second
const sampleLimit = 50000

func makeReceiversConfig(inputs inputSources, opts BuildOptions) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusAppPods = makePrometheusConfigForPods(opts)
		receiversConfig.PrometheusAppServices = makePrometheusConfigForServices(opts)
	}

	if inputs.runtime {
		receiversConfig.KubeletStats = makeKubeletStatsConfig()
		receiversConfig.SingletonK8sClusterReceiverCreator = makeSingletonK8sClusterReceiverCreatorConfig(opts.AgentNamespace)
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
		Endpoint:           fmt.Sprintf("https://${%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       []MetricGroupType{MetricGroupTypeContainer, MetricGroupTypePod, MetricGroupTypeNode},
		Metrics: KubeletStatsMetricsConfig{
			ContainerCPUUsage:            MetricConfig{Enabled: true},
			ContainerCPUUtilization:      MetricConfig{Enabled: false},
			K8sPodCPUUsage:               MetricConfig{Enabled: true},
			K8sPodCPUUtilization:         MetricConfig{Enabled: false},
			K8sNodeCPUUsage:              MetricConfig{Enabled: true},
			K8sNodeCPUUtilization:        MetricConfig{Enabled: false},
			K8sNodeCPUTime:               MetricConfig{Enabled: false},
			K8sNodeMemoryMajorPageFaults: MetricConfig{Enabled: false},
			K8sNodeMemoryPageFaults:      MetricConfig{Enabled: false},
			K8sNodeNetworkIO:             MetricConfig{Enabled: false},
			K8sNodeNetworkErrors:         MetricConfig{Enabled: false},
		},
	}
}

func makeSingletonK8sClusterReceiverCreatorConfig(gatewayNamespace string) *SingletonK8sClusterReceiverCreator {
	metricsToDrop := K8sClusterMetricsConfig{
		K8sContainerStorageRequest:          MetricConfig{false},
		K8sContainerStorageLimit:            MetricConfig{false},
		K8sContainerEphemeralStorageRequest: MetricConfig{false},
		K8sContainerEphemeralStorageLimit:   MetricConfig{false},
		K8sContainerRestarts:                MetricConfig{false},
		K8sContainerReady:                   MetricConfig{false},
		K8sNamespacePhase:                   MetricConfig{false},
		K8sReplicationControllerAvailable:   MetricConfig{false},
		K8sReplicationControllerDesired:     MetricConfig{false},
	}

	return &SingletonK8sClusterReceiverCreator{
		AuthType: "serviceAccount",
		LeaderElection: metric.LeaderElection{
			LeaseName:      "telemetry-metric-gateway-k8scluster",
			LeaseNamespace: gatewayNamespace,
		},
		SingletonK8sClusterReceiver: SingletonK8sClusterReceiver{
			K8sClusterReceiver: K8sClusterReceiver{
				AuthType:               "serviceAccount",
				CollectionInterval:     "30s",
				NodeConditionsToReport: []string{},
				Metrics:                metricsToDrop,
			},
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
