package metricpipeline

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
)

func IsIstioInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio != nil && input.Istio.Enabled != nil && *input.Istio.Enabled
}

func IsEnvoyMetricsEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio.EnvoyMetrics != nil && input.Istio.EnvoyMetrics.Enabled != nil && *input.Istio.EnvoyMetrics.Enabled
}

func IsPrometheusInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus != nil && input.Prometheus.Enabled != nil && *input.Prometheus.Enabled
}

func IsRuntimeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Runtime != nil && input.Runtime.Enabled != nil && *input.Runtime.Enabled
}

func IsOTLPInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.OTLP == nil || !input.OTLP.Disabled
}

func IsPrometheusDiagnosticInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Prometheus.DiagnosticMetrics != nil && input.Prometheus.DiagnosticMetrics.Enabled != nil && *input.Prometheus.DiagnosticMetrics.Enabled
}

func IsIstioDiagnosticInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	return input.Istio.DiagnosticMetrics != nil && input.Istio.DiagnosticMetrics.Enabled != nil && *input.Istio.DiagnosticMetrics.Enabled
}

func IsRuntimePodInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime pod metrics should be enabled by default if any of the fields (Resources, Pod or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Pod == nil || input.Runtime.Resources.Pod.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.Pod.Enabled
}

func IsRuntimeContainerInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime container metrics should be enabled by default if any of the fields (Resources, Container or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Container == nil || input.Runtime.Resources.Container.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.Container.Enabled
}

func IsRuntimeNodeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime node metrics should be enabled by default if any of the fields (Resources, Node or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Node == nil || input.Runtime.Resources.Node.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.Node.Enabled
}

func IsRuntimeVolumeInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime volume metrics should be enabled by default if any of the fields (Resources, Volume or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Volume == nil || input.Runtime.Resources.Volume.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.Volume.Enabled
}

func IsRuntimeStatefulSetInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime Statefulset metrics should be enabled by default if any of the fields (Resources, Statefulset or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.StatefulSet == nil || input.Runtime.Resources.StatefulSet.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.StatefulSet.Enabled
}

func IsRuntimeDeploymentInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime Deployment metrics should be enabled by default if any of the fields (Resources, Deployment or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Deployment == nil || input.Runtime.Resources.Deployment.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.Deployment.Enabled
}

func IsRuntimeDaemonSetInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime DaemonSet metrics should be enabled by default if any of the fields (Resources, DaemonSet or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.DaemonSet == nil || input.Runtime.Resources.DaemonSet.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.DaemonSet.Enabled
}

func IsRuntimeJobInputEnabled(input telemetryv1alpha1.MetricPipelineInput) bool {
	// Runtime Job metrics should be enabled by default if any of the fields (Resources, Job or Enabled) is nil
	if input.Runtime.Resources == nil || input.Runtime.Resources.Job == nil || input.Runtime.Resources.Job.Enabled == nil {
		return true
	}

	return *input.Runtime.Resources.Job.Enabled
}

var schemeToPort map[string]string = map[string]string{
	"http":  "80",
	"https": "443",
}

// OTLPOutputPorts returns the list of ports of the backends defined in all given MetricPipelines
func OTLPOutputPorts(ctx context.Context, c client.Reader, allPipelines []telemetryv1alpha1.MetricPipeline) ([]string, error) {
	backendPorts := []string{}

	for _, pipeline := range allPipelines {
		endpoint, err := common.ResolveValue(ctx, c, pipeline.Spec.Output.OTLP.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve the value of the OTLP output endpoint: %w", err)
		}

		port := extractPort(string(endpoint))

		if port != "" {
			backendPorts = append(backendPorts, port)
		}

		// List of ports needs to be sorted
		// Otherwise, metric agent will continuously restart, because in each reconciliation we can have the ports list in a different order
		slices.Sort(backendPorts)
		// Remove duplication in ports in case multiple backends are defined with the same port
		backendPorts = slices.Compact(backendPorts)
	}

	return backendPorts, nil
}

func extractPort(s string) string {
	normalizedURL := s
	hasScheme := strings.Contains(s, "://")

	// adds a scheme if there are none, since url.Parse only accepts valid URLs
	// without scheme, url.Parse assumes the whole string is the host
	if !hasScheme {
		dummyScheme := "plhd://"
		normalizedURL = dummyScheme + s
	}

	endpoint, err := url.Parse(normalizedURL)
	if err != nil {
		return ""
	}

	if endpoint.Port() == "" {
		port, ok := schemeToPort[endpoint.Scheme]
		if !ok {
			return ""
		}

		return port
	}

	return endpoint.Port()
}
