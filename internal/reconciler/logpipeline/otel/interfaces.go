package otel

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/logagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/loggateway"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// GatewayConfigBuilder builds the OTel Collector configuration for the log gateway.
// The gateway receives logs from agents and forwards them to configured outputs.
type GatewayConfigBuilder interface {
	// Build generates the collector configuration, environment variables, and any errors encountered.
	// It takes all log pipelines and build options including cluster information and enrichment settings.
	Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, opts loggateway.BuildOptions) (*common.Config, common.EnvVars, error)
}

// GatewayApplierDeleter manages the lifecycle of log gateway Kubernetes resources.
// It handles both creation/updates and cleanup of gateway deployments, services, and related resources.
type GatewayApplierDeleter interface {
	// ApplyResources creates or updates all gateway resources using the provided configuration.
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error

	// DeleteResources removes all gateway resources. The isIstioActive flag determines
	// whether Istio-specific resources should be cleaned up.
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

// GatewayFlowHealthProber checks the health of log flow through the gateway.
// It probes whether logs are successfully flowing from the gateway to the backend.
type GatewayFlowHealthProber interface {
	// Probe checks if logs are flowing correctly for a specific pipeline.
	// Returns probe results containing health status and metrics.
	Probe(ctx context.Context, pipelineName string) (prober.OTelGatewayProbeResult, error)
}

// AgentFlowHealthProber checks the health of log flow through the agent.
// It probes whether logs are successfully flowing from the agent to the gateway.
type AgentFlowHealthProber interface {
	// Probe checks if logs are flowing correctly for a specific pipeline.
	// Returns probe results containing health status and metrics.
	Probe(ctx context.Context, pipelineName string) (prober.OTelAgentProbeResult, error)
}

// IstioStatusChecker determines if Istio service mesh is active in the cluster.
// This affects resource configuration, particularly network policies and sidecars.
type IstioStatusChecker interface {
	// IsIstioActive returns true if Istio is installed and active in the cluster.
	IsIstioActive(ctx context.Context) bool
}

// AgentConfigBuilder builds the OTel Collector configuration for the log agent.
// The agent runs as a DaemonSet and collects logs from application containers.
type AgentConfigBuilder interface {
	// Build generates the collector configuration, environment variables, and any errors encountered.
	// It takes all log pipelines requiring agents and build options including cluster information.
	Build(ctx context.Context, pipelines []telemetryv1alpha1.LogPipeline, options logagent.BuildOptions) (*common.Config, common.EnvVars, error)
}

// AgentApplierDeleter manages the lifecycle of log agent Kubernetes resources.
// It handles both creation/updates and cleanup of agent DaemonSets, ConfigMaps, and related resources.
type AgentApplierDeleter interface {
	// ApplyResources creates or updates all agent resources using the provided configuration.
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.AgentApplyOptions) error

	// DeleteResources removes all agent resources from the cluster.
	DeleteResources(ctx context.Context, c client.Client) error
}

// Prober checks the readiness of Kubernetes workloads (Deployments, DaemonSets).
// It verifies that pods are running and healthy before marking components as ready.
type Prober interface {
	// IsReady checks if the workload identified by name is ready.
	// Returns an error if the workload is not ready, with details about the failure.
	IsReady(ctx context.Context, name types.NamespacedName) error
}

// ErrorToMessageConverter converts errors to human-readable messages for status conditions.
// It provides consistent, user-friendly error messages across the reconciler.
type ErrorToMessageConverter interface {
	// Convert transforms an error into a descriptive message suitable for display in status conditions.
	Convert(err error) string
}
