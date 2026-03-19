package common

// Label Architecture
//
// Labels are applied to Kubernetes resources through two mechanisms:
//
//  1. Labeler interceptor (k8sclients.NewLabeler): Automatically stamps DefaultLabels
//     on every top-level object metadata during Create/Update/Patch. This covers
//     Deployments, DaemonSets, Services, ConfigMaps, NetworkPolicies, etc.
//
//  2. Explicit assignment in resource builders: Required for labels that the Labeler
//     cannot reach or that are resource-specific. This includes:
//     - Pod template labels (nested inside workloads, invisible to the Labeler)
//     - Selector labels (must be set explicitly in spec.selector, service.spec.selector, etc.)
//     - Functional pod labels (e.g., sidecar.istio.io/inject to control Istio injection)
//     - Service discovery labels (e.g., telemetry.kyma-project.io/self-monitor on services
//     to mark them for scraping by the self-monitor)
//     - NetworkPolicy traffic labels (e.g., telemetry.kyma-project.io/log-ingest on pods
//     to allow traffic matching in deny-all setups)
//     - User-provided labels from globals.AdditionalWorkloadLabels()
//
// Label hierarchy (each level includes all labels from the level above):
//
//	ModuleLabels (static)
//	  kyma-project.io/module: telemetry
//	  app.kubernetes.io/part-of: telemetry
//	  app.kubernetes.io/managed-by: telemetry-manager
//
//	DefaultLabels = ModuleLabels + name + component
//	  app.kubernetes.io/name: <baseName>
//	  app.kubernetes.io/component: <agent|gateway|monitor|controller>
//
//	DefaultSelector (1 label) ⊂ DefaultLabels
//	  app.kubernetes.io/name: <baseName>
//	  Used for: spec.selector in Deployments/DaemonSets, spec.selector in Services,
//	  spec.podSelector in NetworkPolicies, pod anti-affinity, PeerAuthentication
//
// Who applies what:
//
//		Top-level object labels:      Labeler stamps DefaultLabels automatically
//		Pod template labels:          Builders set DefaultLabels + functional labels + user labels explicitly
//		Selector labels:              Builders set DefaultSelector explicitly
//	    Additional resource labels:   Builders set labels explicitly
//		Additional user labels:       Builders copy from globals.AdditionalWorkloadLabels()

const (
	// LabelValueTrue can be used in all labels that require "true" as value
	LabelValueTrue = "true"
	// LabelValueFalse can be used in all labels that require "false" as value
	LabelValueFalse = "false"

	LabelKeyKymaModule            = "kyma-project.io/module"
	LabelValueKymaModuleTelemetry = "telemetry"

	LabelKeyK8sName                        = "app.kubernetes.io/name"
	LabelKeyK8sPartOf                      = "app.kubernetes.io/part-of"
	LabelValueK8sPartOfTelemetry           = "telemetry"
	LabelKeyK8sManagedBy                   = "app.kubernetes.io/managed-by"
	LabelValueK8sManagedByTelemetryManager = "telemetry-manager"
	LabelKeyK8sComponent                   = "app.kubernetes.io/component"
	LabelValueK8sComponentController       = "controller"
	LabelValueK8sComponentAgent            = "agent"
	LabelValueK8sComponentGateway          = "gateway"
	LabelValueK8sComponentMonitor          = "monitor"
	LabelKeyK8sInstance                    = "app.kubernetes.io/instance"
	LabelValueK8sInstanceTelemetry         = "telemetry"
	LabelKeyK8sHostname                    = "kubernetes.io/hostname"
	LabelKeyK8sZone                        = "topology.kubernetes.io/zone"

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
		LabelKeyKymaModule:   LabelValueKymaModuleTelemetry,
		LabelKeyK8sPartOf:    LabelValueK8sPartOfTelemetry,
		LabelKeyK8sManagedBy: LabelValueK8sManagedByTelemetryManager,
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
