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
		receiversConfig.KubeletStats = makeKubeletStatsConfig(inputs.runtimeResources)
		receiversConfig.SingletonK8sClusterReceiverCreator = makeSingletonK8sClusterReceiverCreatorConfig(opts.AgentNamespace, inputs.runtimeResources)
	}

	if inputs.istio {
		receiversConfig.PrometheusIstio = makePrometheusIstioConfig()
	}

	return receiversConfig
}

func makeKubeletStatsConfig(runtimeResources runtimeResourcesEnabled) *KubeletStatsReceiver {
	const (
		collectionInterval = "30s"
		portKubelet        = 10250
	)

	return &KubeletStatsReceiver{
		CollectionInterval: collectionInterval,
		AuthType:           "serviceAccount",
		InsecureSkipVerify: true,
		Endpoint:           fmt.Sprintf("https://${%s}:%d", config.EnvVarCurrentNodeName, portKubelet),
		MetricGroups:       makeKubeletStatsMetricGroups(runtimeResources),
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
		ExtraMetadataLabels: []string{"k8s.volume.type"},
	}
}

func makeSingletonK8sClusterReceiverCreatorConfig(gatewayNamespace string, runtimeResources runtimeResourcesEnabled) *SingletonK8sClusterReceiverCreator {

	return &SingletonK8sClusterReceiverCreator{
		AuthType: "serviceAccount",
		LeaderElection: metric.LeaderElection{
			LeaseName:      "telemetry-metric-agent-k8scluster",
			LeaseNamespace: gatewayNamespace,
		},
		SingletonK8sClusterReceiver: SingletonK8sClusterReceiver{
			K8sClusterReceiver: K8sClusterReceiver{
				AuthType:               "serviceAccount",
				CollectionInterval:     "30s",
				NodeConditionsToReport: []string{},
				Metrics:                makeK8sClusterMetricsToDrop(runtimeResources),
			},
		},
	}
}

func makeK8sClusterMetricsToDrop(runtimeResources runtimeResourcesEnabled) K8sClusterMetricsToDrop {

	metricsToDrop := K8sClusterMetricsToDrop{}

	metricsToDrop.K8sClusterDefaultMetricsToDrop = &K8sClusterDefaultMetricsToDrop{
		K8sContainerStorageRequest:          MetricConfig{Enabled: false},
		K8sContainerStorageLimit:            MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageRequest: MetricConfig{Enabled: false},
		K8sContainerEphemeralStorageLimit:   MetricConfig{Enabled: false},
		K8sContainerRestarts:                MetricConfig{Enabled: false},
		K8sContainerReady:                   MetricConfig{Enabled: false},
		K8sNamespacePhase:                   MetricConfig{Enabled: false},
		K8sHPACurrentReplicas:               MetricConfig{Enabled: false},
		K8sHPADesiredReplicas:               MetricConfig{Enabled: false},
		K8sHPAMinReplicas:                   MetricConfig{Enabled: false},
		K8sHPAMaxReplicas:                   MetricConfig{Enabled: false},
		K8sReplicaSetAvailable:              MetricConfig{Enabled: false},
		K8sReplicaSetDesired:                MetricConfig{Enabled: false},
		K8sReplicationControllerAvailable:   MetricConfig{Enabled: false},
		K8sReplicationControllerDesired:     MetricConfig{Enabled: false},
		K8sResourceQuotaHardLimit:           MetricConfig{Enabled: false},
		K8sResourceQuotaUsed:                MetricConfig{Enabled: false},
	}

	// The following metrics are enabled by default in the K8sClusterReceiver. If we disable these resources in
	//pipeline config we need to disable the corresponding metrics in the K8sClusterReceiver.
	if !runtimeResources.statefulSet {
		metricsToDrop.K8sClusterStatefulSetMetricsToDrop = &K8sClusterStatefulSetMetricsToDrop{
			K8sStatefulSetCurrentPods: MetricConfig{false},
			K8sStatefulSetDesiredPods: MetricConfig{false},
			K8sStatefulSetReadyPods:   MetricConfig{false},
			K8sStatefulSetUpdatedPods: MetricConfig{false},
		}
	}

	if !runtimeResources.job {
		metricsToDrop.K8sClusterJobMetricsToDrop = &K8sClusterJobMetricsToDrop{
			K8sJobActiveJobs:            MetricConfig{false},
			K8sJobDesiredSuccessfulPods: MetricConfig{false},
			K8sJobFailedPods:            MetricConfig{false},
			K8sJobMaxParallelPods:       MetricConfig{false},
		}
	}

	if !runtimeResources.deployment {
		metricsToDrop.K8sClusterDeploymentMetricsToDrop = &K8sClusterDeploymentMetricsToDrop{
			K8sDeploymentAvailable: MetricConfig{false},
			K8sDeploymentDesired:   MetricConfig{false},
		}
	}

	if !runtimeResources.daemonSet {
		metricsToDrop.K8sClusterDaemonSetMetricsToDrop = &K8sClusterDaemonSetMetricsToDrop{
			K8sDaemonSetCurrentScheduledNodes: MetricConfig{false},
			K8sDaemonSetDesiredScheduledNodes: MetricConfig{false},
			K8sDaemonSetMisscheduledNodes:     MetricConfig{false},
			K8sDaemonSetReadyNodes:            MetricConfig{false},
		}
	}

	return metricsToDrop
}

func makeKubeletStatsMetricGroups(runtimeResources runtimeResourcesEnabled) []MetricGroupType {
	var metricGroups []MetricGroupType

	if runtimeResources.container {
		metricGroups = append(metricGroups, MetricGroupTypeContainer)
	}

	if runtimeResources.pod {
		metricGroups = append(metricGroups, MetricGroupTypePod)
	}

	if runtimeResources.node {
		metricGroups = append(metricGroups, MetricGroupTypeNode)
	}

	if runtimeResources.volume {
		metricGroups = append(metricGroups, MetricGroupTypeVolume)
	}

	return metricGroups
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
