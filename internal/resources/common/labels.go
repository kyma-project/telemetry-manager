package common

const (
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

	LabelKeyTelemetryLogIngest     = "telemetry.kyma-project.io/log-ingest"
	LabelKeyTelemetryLogExport     = "telemetry.kyma-project.io/log-export"
	LabelKeyTelemetryTraceIngest   = "telemetry.kyma-project.io/trace-ingest"
	LabelKeyTelemetryTraceExport   = "telemetry.kyma-project.io/trace-export"
	LabelKeyTelemetryMetricIngest  = "telemetry.kyma-project.io/metric-ingest"
	LabelKeyTelemetryMetricExport  = "telemetry.kyma-project.io/metric-export"
	LabelKeyTelemetryMetricScrape  = "telemetry.kyma-project.io/metric-scrape"
	LabelKeyTelemetrySelfMonitor   = "telemetry.kyma-project.io/self-monitor"
	LabelValueTelemetrySelfMonitor = "enabled"
)

func MakeDefaultLabels(baseName string, componentLabelValue string) map[string]string {
	return map[string]string{
		LabelKeyK8sName:      baseName,
		LabelKeyKymaModule:   LabelValueKymaModule,
		LabelKeyK8sPartOf:    LabelValueK8sPartOf,
		LabelKeyK8sManagedBy: LabelValueK8sManagedBy,
		LabelKeyK8sComponent: componentLabelValue,
	}
}

func MakeDefaultSelectorLabels(baseName string) map[string]string {
	return map[string]string{
		LabelKeyK8sName: baseName,
	}
}
