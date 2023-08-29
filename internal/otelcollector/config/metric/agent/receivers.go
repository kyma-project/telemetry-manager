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
	scrapeConfig := ScrapeConfig{
		JobName:                    "app-pods",
		ScrapeInterval:             scrapeInterval,
		KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RolePod}},
		RelabelConfigs: []RelabelConfig{
			keepRunningOnSameNode(NodeAffiliatedPod),
			keepAnnotated(AnnotatedPod),
			replaceScheme(AnnotatedPod),
			replaceMetricPath(AnnotatedPod),
			replaceAddress(AnnotatedPod),
			dropNonRunningPods(),
			dropInitContainers(),
			dropIstioProxyContainer(),
		},
	}

	if isIstioActive {
		scrapeConfig.TLSConfig = &TLSConfig{
			CAFile:             istioCAFile,
			CertFile:           istioCertFile,
			KeyFile:            istioKeyFile,
			InsecureSkipVerify: true,
		}

		scrapeConfig.RelabelConfigs = append(scrapeConfig.RelabelConfigs,
			replaceSchemeIfSidecarFound(),
			dropNonHTTPS(),
		)
	}

	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{scrapeConfig},
		},
	}
}

func makePrometheusAppServicesConfig(isIstioActive bool) *PrometheusReceiver {
	scrapeConfig := ScrapeConfig{
		JobName:                    "app-services",
		ScrapeInterval:             scrapeInterval,
		KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RoleEndpoints}},
		RelabelConfigs: []RelabelConfig{
			keepRunningOnSameNode(NodeAffiliatedEndpoint),
			keepAnnotated(AnnotatedService),
			replaceScheme(AnnotatedService),
			replaceMetricPath(AnnotatedService),
			replaceAddress(AnnotatedService),
			dropNonRunningPods(),
			dropInitContainers(),
			dropIstioProxyContainer(),
			replaceService(),
		},
	}

	if isIstioActive {
		scrapeConfig.TLSConfig = &TLSConfig{
			CAFile:             istioCAFile,
			CertFile:           istioCertFile,
			KeyFile:            istioKeyFile,
			InsecureSkipVerify: true,
		}

		scrapeConfig.RelabelConfigs = append(scrapeConfig.RelabelConfigs,
			replaceSchemeIfSidecarFound(),
			dropNonHTTPS(),
		)
	}

	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{scrapeConfig},
		},
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
						keepRunningOnSameNode(NodeAffiliatedPod),
						keepIstioProxyContainer(),
						keepContainerWithEnvoyPort(),
						dropNonRunningPods(),
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
