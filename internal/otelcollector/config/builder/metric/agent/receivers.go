package agent

import (
	"fmt"
	"time"

	"github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/builder/common"
	promcommonconfig "github.com/prometheus/common/config"
	prommodel "github.com/prometheus/common/model"
	promconfig "github.com/prometheus/prometheus/config"
	promdiscovery "github.com/prometheus/prometheus/discovery"
	promk8sdiscovery "github.com/prometheus/prometheus/discovery/kubernetes"
	promtargetgroup "github.com/prometheus/prometheus/discovery/targetgroup"
	promlabel "github.com/prometheus/prometheus/model/relabel"
	"path"
)

const IstioCertDir = "/etc/istio-output-certs"

func istioCAPath() string {
	return path.Join(IstioCertDir, "root-cert.pem")
}

func istioCertPath() string {
	return path.Join(IstioCertDir, "cert-chain.pem")
}

func istioKeyPath() string {
	return path.Join(IstioCertDir, "key.pem")
}

func makeReceiversConfig(pipelines []v1alpha1.MetricPipeline) config.ReceiversConfig {
	enableRuntimeMetrics := false
	for i := range pipelines {
		input := pipelines[i].Spec.Input
		if input.Application.Runtime.Enabled {
			enableRuntimeMetrics = true
		}
	}

	var receiversConfig config.ReceiversConfig
	receiversConfig.PrometheusSelf = makePrometheusSelfConfig()
	receiversConfig.PrometheusAppPods = makePrometheusAppPodsConfig()
	if enableRuntimeMetrics {
		receiversConfig.KubeletStats = makeKubeletStatsConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig() *config.KubeletStatsReceiverConfig {
	const collectionInterval = "30s"
	const portKubelet = 10250
	return &config.KubeletStatsReceiverConfig{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		Endpoint:           fmt.Sprintf("https://${env:%s}:%d", common.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       []config.MetricGroupType{config.MetricGroupTypeContainer, config.MetricGroupTypePod},
	}
}

func makePrometheusSelfConfig() *config.PrometheusReceiverConfig {
	targets := []*promtargetgroup.Group{
		{
			Targets: []prommodel.LabelSet{
				{
					prommodel.AddressLabel: prommodel.LabelValue(fmt.Sprintf("${%s}:%d", common.EnvVarCurrentPodIP, common.PortMetrics)),
				},
			},
		},
	}

	return &config.PrometheusReceiverConfig{
		Config: promconfig.Config{
			ScrapeConfigs: []*promconfig.ScrapeConfig{
				{
					JobName:          "opentelemetry-collector",
					ScrapeInterval:   prommodel.Duration(10 * time.Second),
					HTTPClientConfig: promcommonconfig.DefaultHTTPClientConfig,
					ServiceDiscoveryConfigs: []promdiscovery.Config{
						promdiscovery.StaticConfig(targets),
					},
				},
			},
		},
	}
}

func makePrometheusAppPodsConfig() *config.PrometheusReceiverConfig {
	return &config.PrometheusReceiverConfig{
		Config: promconfig.Config{
			ScrapeConfigs: []*promconfig.ScrapeConfig{
				{
					JobName:        "app-pods",
					ScrapeInterval: prommodel.Duration(10 * time.Second),
					ServiceDiscoveryConfigs: []promdiscovery.Config{
						&promk8sdiscovery.SDConfig{
							Role:             promk8sdiscovery.RolePod,
							HTTPClientConfig: promcommonconfig.DefaultHTTPClientConfig,
						},
					},
					HTTPClientConfig: promcommonconfig.HTTPClientConfig{
						TLSConfig: promcommonconfig.TLSConfig{
							CAFile:             istioCAPath(),
							CertFile:           istioCertPath(),
							KeyFile:            istioKeyPath(),
							InsecureSkipVerify: true,
						},
					},
					RelabelConfigs: []*promlabel.Config{
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_node_name"},
							Regex:        promlabel.MustNewRegexp(fmt.Sprintf("$%s", common.EnvVarCurrentNodeName)),
							Action:       promlabel.Keep,
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_namespace"},
							Regex:        promlabel.MustNewRegexp("(kyma-system|kube-system)"),
							Action:       promlabel.Drop,
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_scrape"},
							Regex:        promlabel.MustNewRegexp("true"),
							Action:       promlabel.Keep,
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_label_security_istio_io_tlsMode"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("(istio)"),
							Replacement:  "https",
							TargetLabel:  "__scheme__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_scheme"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("(https?)"),
							TargetLabel:  "__scheme__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_path"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("(.+)"),
							TargetLabel:  "__metrics_path__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__address__", "__meta_kubernetes_pod_annotation_prometheus_io_port"},
							Action:       promlabel.Replace,
							Regex:        promlabel.MustNewRegexp("([^:]+)(?::\\d+)?;(\\d+)"),
							Replacement:  "$$1:$$2",
							TargetLabel:  "__address__",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_namespace"},
							Action:       promlabel.Replace,
							TargetLabel:  "namespace",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_name"},
							Action:       promlabel.Replace,
							TargetLabel:  "pod",
						},
						{
							SourceLabels: []prommodel.LabelName{"__meta_kubernetes_pod_node_name"},
							Action:       promlabel.Replace,
							TargetLabel:  "node",
						},
					},
				},
			},
		},
	}
}
