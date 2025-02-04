package common

const (
	KymaModuleLabelKey   = "kyma-project.io/module"
	KymaModuleLabelValue = "telemetry"

	K8sNameLabelKey                  = "app.kubernetes.io/name"
	K8sPartOfLabelKey                = "app.kubernetes.io/part-of"
	K8sPartOfLabelValue              = "telemetry"
	K8sManagedByLabelKey             = "app.kubernetes.io/managed-by"
	K8sManagedByLabelValue           = "telemetry-manager"
	K8sComponentLabelKey             = "app.kubernetes.io/component"
	K8sComponentLabelValueController = "controller"
	K8sComponentLabelValueAgent      = "agent"
	K8sComponentLabelValueGateway    = "gateway"
	K8sComponentLabelValueMonitor    = "monitor"
	K8sInstanceLabelKey              = "app.kubernetes.io/instance"
	K8sInstanceLabelValue            = "telemetry"
	K8sHostnameLabelKey              = "kubernetes.io/hostname"
	K8sZoneLabelKey                  = "topology.kubernetes.io/zone"

	IstioInjectLabelKey = "sidecar.istio.io/inject"

	TelemetryLogIngestLabelKey     = "telemetry.kyma-project.io/log-ingest"
	TelemetryLogExportLabelKey     = "telemetry.kyma-project.io/log-export"
	TelemetryTraceIngestLabelKey   = "telemetry.kyma-project.io/trace-ingest"
	TelemetryTraceExportLabelKey   = "telemetry.kyma-project.io/trace-export"
	TelemetryMetricIngestLabelKey  = "telemetry.kyma-project.io/metric-ingest"
	TelemetryMetricExportLabelKey  = "telemetry.kyma-project.io/metric-export"
	TelemetryMetricScrapeLabelKey  = "telemetry.kyma-project.io/metric-scrape"
	TelemetrySelfMonitorLabelKey   = "telemetry.kyma-project.io/self-monitor"
	TelemetrySelfMonitorLabelValue = "enabled"
)

func MakeDefaultLabels(baseName string, componentLabelValue string) map[string]string {
	return map[string]string{
		K8sNameLabelKey:      baseName,
		KymaModuleLabelKey:   KymaModuleLabelValue,
		K8sPartOfLabelKey:    K8sPartOfLabelValue,
		K8sManagedByLabelKey: K8sManagedByLabelValue,
		K8sComponentLabelKey: componentLabelValue,
	}
}

func MakeDefaultSelectorLabels(baseName string) map[string]string {
	return map[string]string{
		K8sNameLabelKey: baseName,
	}
}
