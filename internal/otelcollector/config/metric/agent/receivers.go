package agent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

const scrapeInterval = 30 * time.Second
const istioCAFile = "/etc/istio-output-certs/root-cert.pem"
const istioCertFile = "/etc/istio-output-certs/cert-chain.pem"
const istioKeyFile = "/etc/istio-output-certs/key.pem"

func makeReceiversConfig(inputs inputSources, istioDeployed bool) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusSelf = makePrometheusSelfConfig()
		receiversConfig.PrometheusAppPods = makePrometheusAppPodsConfig(istioDeployed)
		receiversConfig.PrometheusAppServices = makePrometheusAppServicesConfig(istioDeployed)
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

func makePrometheusAppPodsConfig(istioDeployed bool) *PrometheusReceiver {
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

	if istioDeployed {
		scrapeConfig.TLSConfig = &TLSConfig{
			CAFile:             istioCAFile,
			CertFile:           istioCertFile,
			KeyFile:            istioKeyFile,
			InsecureSkipVerify: true,
		}

		scrapeConfig.RelabelConfigs = append(scrapeConfig.RelabelConfigs, replaceSchemeIstioTLS(), dropNonHTTPS())
	}

	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{scrapeConfig},
		},
	}
}

func makePrometheusAppServicesConfig(istioDeployed bool) *PrometheusReceiver {
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

	if istioDeployed {
		scrapeConfig.TLSConfig = &TLSConfig{
			CAFile:             istioCAFile,
			CertFile:           istioCertFile,
			KeyFile:            istioKeyFile,
			InsecureSkipVerify: true,
		}

		scrapeConfig.RelabelConfigs = append(scrapeConfig.RelabelConfigs, replaceSchemeIstioTLS(), dropNonHTTPS())
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
