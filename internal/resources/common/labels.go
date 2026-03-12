package common

const (
	// LabelValueTrue can be used in all labels that require "true" as value
	LabelValueTrue = "true"
	// LabelValueFalse can be used in all labels that require "false" as value
	LabelValueFalse = "false"

	LabelKeyKymaModule   = "kyma-project.io/module"
	LabelValueKymaModule = "telemetry"

	LabelKeyK8sName                  = "app.kubernetes.io/name"
	LabelKeyK8sPartOf                = "app.kubernetes.io/part-of"
	LabelValueK8sPartOf              = "telemetry"
	LabelKeyK8sManagedBy             = "app.kubernetes.io/managed-by"
	LabelValueK8sManagedBy           = "telemetry-manager"
	LabelKeyK8sComponent             = "app.kubernetes.io/component"
	LabelValueK8sComponentController = "controller"
	LabelValueK8sComponentAgent      = "agent"
	LabelValueK8sComponentGateway    = "gateway"
	LabelValueK8sComponentMonitor    = "monitor"
	LabelKeyK8sInstance              = "app.kubernetes.io/instance"
	LabelValueK8sInstance            = "telemetry"
	LabelKeyK8sHostname              = "kubernetes.io/hostname"
	LabelKeyK8sZone                  = "topology.kubernetes.io/zone"

	LabelKeyIstioInject = "sidecar.istio.io/inject"

	LabelKeyTelemetrySelfMonitor   = "telemetry.kyma-project.io/self-monitor"
	LabelValueTelemetrySelfMonitor = "enabled"

	// The labels below can be used by a NetworkPolicy to allow traffic to/from components of the telemetry module in a deny-all traffic setup

	LabelKeyTelemetryLogIngest    = "telemetry.kyma-project.io/log-ingest"
	LabelKeyTelemetryLogExport    = "telemetry.kyma-project.io/log-export"
	LabelKeyTelemetryTraceIngest  = "telemetry.kyma-project.io/trace-ingest"
	LabelKeyTelemetryTraceExport  = "telemetry.kyma-project.io/trace-export"
	LabelKeyTelemetryMetricIngest = "telemetry.kyma-project.io/metric-ingest"
	LabelKeyTelemetryMetricExport = "telemetry.kyma-project.io/metric-export"
	// NOTE: The labels "telemetry.kyma-project.io/metric-scrape" and "networking.kyma-project.io/metrics-scraping" have similar names, but different purposes as described below:

	// LabelKeyTelemetryMetricScrape can be used by a NetworkPolicy to allow the metric agent to scrape metrics from user workloads in a deny-all ingress traffic setup
	// Check https://kyma-project.io/external-content/telemetry-manager/docs/user/troubleshooting.html#metricpipeline-failed-to-scrape-prometheus-endpoint for the troubleshooting guide using this label
	LabelKeyTelemetryMetricScrape = "telemetry.kyma-project.io/metric-scrape"
	// LabelKeyTelemetryMetricsScraping is required to allow the metric agent to scrape metrics from Kyma modules
	// Check https://github.com/kyma-project/kyma/issues/18818 for more details
	LabelKeyTelemetryMetricsScraping   = "networking.kyma-project.io/metrics-scraping"
	LabelValueTelemetryMetricsScraping = "allowed"
)

// ModuleLabels returns the base labels that identify a resource as belonging to the telemetry module.
// These labels are shared by all telemetry-managed resources regardless of component type.
func ModuleLabels() map[string]string {
	return map[string]string{
		LabelKeyKymaModule:   LabelValueKymaModule,
		LabelKeyK8sPartOf:    LabelValueK8sPartOf,
		LabelKeyK8sManagedBy: LabelValueK8sManagedBy,
	}
}

// DefaultLabels returns the standard set of labels for a telemetry component resource.
// It extends ModuleLabels with the component's base name and type (e.g., "agent", "gateway", "monitor").
func DefaultLabels(componentBaseName string, componentType string) map[string]string {
	l := ModuleLabels()
	l[LabelKeyK8sName] = componentBaseName
	l[LabelKeyK8sComponent] = componentType

	return l
}

// DefaultSelector returns the minimal label set used for pod selectors in Deployments, DaemonSets,
// Services, and NetworkPolicies. This must be a subset of DefaultLabels to ensure selectors match.
func DefaultSelector(componentBaseName string) map[string]string {
	return map[string]string{
		LabelKeyK8sName: componentBaseName,
	}
}
