package labels

const (
	selectorLabelKey            = "app.kubernetes.io/name"
	traceGatewayIngestSelector  = "telemetry.kyma-project.io/trace-ingest"
	traceGatewayExportSelector  = "telemetry.kyma-project.io/trace-export"
	metricAgentScrapeSelector   = "telemetry.kyma-project.io/metric-scrape"
	metricGatewayIngestSelector = "telemetry.kyma-project.io/metric-ingest"
	metricGatewayExportSelector = "telemetry.kyma-project.io/metric-export"
	istioSidecarInjectLabel     = "sidecar.istio.io/inject"
)

func MakeDefaultLabel(baseName string) map[string]string {
	return map[string]string{
		selectorLabelKey: baseName,
	}
}

func MakeMetricAgentSelectorLabel(baseName string) map[string]string {
	return map[string]string{
		selectorLabelKey:          baseName,
		metricAgentScrapeSelector: "true",
		istioSidecarInjectLabel:   "true",
	}
}

func MakeMetricGatewaySelectorLabel(baseName string) map[string]string {
	return map[string]string{
		selectorLabelKey:            baseName,
		metricGatewayIngestSelector: "true",
		metricGatewayExportSelector: "true",
		istioSidecarInjectLabel:     "true",
	}
}

func MakeTraceGatewaySelectorLabel(baseName string) map[string]string {
	return map[string]string{
		selectorLabelKey:           baseName,
		traceGatewayIngestSelector: "true",
		traceGatewayExportSelector: "true",
		istioSidecarInjectLabel:    "true",
	}
}
