package agent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
)

const scrapeInterval = 30 * time.Second

func makeReceiversConfig(inputs inputSources) Receivers {
	var receiversConfig Receivers

	if inputs.prometheus {
		receiversConfig.PrometheusSelf = makePrometheusSelfConfig()
		receiversConfig.PrometheusAppPods = makePrometheusAppPodsConfig(inputs.istioDeployed)
		receiversConfig.PrometheusAppServices = makePrometheusAppServicesConfig(inputs.istioDeployed)
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

func makePrometheusAppPodsConfig(istioEnabled bool) *PrometheusReceiver {
	var tlsConfig = TLSConfig{}
	if istioEnabled {
		tlsConfig = TLSConfig{
			CAFile:             "/etc/istio-output-certs/root-cert.pem",
			CertFile:           "/etc/istio-output-certs/cert-chain.pem",
			KeyFile:            "key_file: /etc/istio-output-certs/key.pem",
			InsecureSkipVerify: true,
		}
	}
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
					JobName:                    "app-pods",
					ScrapeInterval:             scrapeInterval,
					KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{Role: RolePod}},
					TLSConfig:                  tlsConfig,
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
				},
			},
		},
	}
}

func makePrometheusAppServicesConfig(istioEnabled bool) *PrometheusReceiver {
	return &PrometheusReceiver{
		Config: PrometheusConfig{
			ScrapeConfigs: []ScrapeConfig{
				{
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
				},
			},
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
