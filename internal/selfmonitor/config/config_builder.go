package config

import (
	"strings"
	"time"
)

const defaultInterval = 30 * time.Second

type BuilderConfig struct {
	ScrapeNamespace   string
	WebhookURL        string
	WebhookScheme     string
	ConfigPath        string
	AlertRuleFileName string
}

func MakeConfig(builderCfg BuilderConfig) Config {
	promConfig := Config{}
	promConfig.GlobalConfig = makeGlobalConfig()
	promConfig.AlertingConfig = makeAlertConfig(builderCfg.WebhookURL, builderCfg.WebhookScheme)
	promConfig.RuleFiles = []string{builderCfg.ConfigPath + builderCfg.AlertRuleFileName}
	promConfig.ScrapeConfigs = makeScrapeConfig(builderCfg.ScrapeNamespace)

	return promConfig
}

func makeGlobalConfig() GlobalConfig {
	return GlobalConfig{
		ScraperInterval:    defaultInterval,
		EvaluationInterval: defaultInterval,
	}
}

func makeAlertConfig(webhookURL, webhookScheme string) AlertingConfig {
	return AlertingConfig{
		AlertManagers: []AlertManagerConfig{{
			Scheme: webhookScheme,
			StaticConfigs: []AlertManagerStaticConfig{{
				Targets: []string{webhookURL},
			}},
			TLSConfig: TLSConfig{
				InsecureSkipVerify: true,
			},
		}},
	}
}

func makeScrapeConfig(scrapeNamespace string) []ScrapeConfig {
	return []ScrapeConfig{
		{
			JobName: "kubernetes-service-endpoints",
			RelabelConfigs: []RelabelConfig{
				{
					SourceLabels: []string{"__meta_kubernetes_namespace"},
					Action:       Keep,
					Regex:        scrapeNamespace,
				},
				{
					SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_scrape"},
					Action:       Keep,
					Regex:        "true",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_endpoints_label_telemetry_kyma_project_io_self_monitor"},
					Action:       Keep,
					Regex:        "enabled",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_service_annotation_prometheus_io_path"},
					Action:       Replace,
					TargetLabel:  "__metrics_path__",
					Regex:        "(.+)",
				},
				{
					SourceLabels: []string{"__address__", "__meta_kubernetes_service_annotation_prometheus_io_port"},
					Action:       Replace,
					TargetLabel:  "__address__",
					Regex:        "(.+?)(?::\\d+)?;(\\d+)",
					Replacement:  "$1:$2",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_namespace"},
					Action:       Replace,
					TargetLabel:  "namespace",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_service_name"},
					Action:       Replace,
					TargetLabel:  "service",
				},
				{
					SourceLabels: []string{"__meta_kubernetes_pod_node_name"},
					Action:       Replace,
					TargetLabel:  "node",
				},
			},
			MetricRelabelConfigs: []RelabelConfig{
				{
					SourceLabels: []string{"__name__"},
					Action:       Keep,
					Regex:        scrapableMetricsRegex(),
				},
				// The following relabel configs add an artificial pipeline_name label to the Fluent Bit and OTel Collector metrics to simplify pipeline matching
				// For Fluent Bit metrics, the pipeline_name is based on the name label. Note that a regex group matching Kubernetes resource names (alphanumerical chars and hyphens) is used to extract the pipeline name.
				// It allows to filter out timeseries with technical names (storage_backend.0, tail.0, etc.)
				// For OTel Collector metrics, the pipeline_name is extracted from the exporter label, which has the format [otlp|otlphttp]/<pipeline_name>
				{
					SourceLabels: []string{"__name__", "name"},
					Action:       Replace,
					Regex:        "fluentbit_.+;([a-zA-Z0-9-]+)",
					TargetLabel:  "pipeline_name",
				},
				{
					SourceLabels: []string{"__name__", "exporter"},
					Action:       Replace,
					Regex:        "otelcol_.+;.+/([a-zA-Z0-9-]+)",
					TargetLabel:  "pipeline_name",
				},
			},
			KubernetesDiscoveryConfigs: []KubernetesDiscoveryConfig{{
				Role:       RoleEndpoints,
				Namespaces: Names{Name: []string{scrapeNamespace}},
			}},
		},
	}
}

func scrapableMetricsRegex() string {
	fluentBitMetrics := []string{
		fluentBitOutputProcBytesTotal,
		fluentBitOutputDroppedRecordsTotal,
		fluentBitInputBytesTotal,
		fluentBitBufferUsageBytes,
	}

	otelCollectorMetrics := []string{
		otelExporterSent,
		otelExporterSendFailed,
		otelExporterEnqueueFailed,
		otelReceiverRefused,
	}

	for i := range otelCollectorMetrics {
		otelCollectorMetrics[i] += "_.*"
	}

	// exporter_queue_size and exporter_queue_capacity do not have a suffix
	otelCollectorMetrics = append(otelCollectorMetrics, otelExporterQueueSize, otelExporterQueueCapacity)

	return strings.Join(append(fluentBitMetrics,
		otelCollectorMetrics...), "|")
}
