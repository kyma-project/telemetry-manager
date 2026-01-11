package otel

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/logagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/loggateway"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// GatewayConfigBuilder builds the OTel Collector configuration for the log gateway.
// The gateway receives logs from agents and forwards them to configured outputs.
type GatewayConfigBuilder interface {
	// Build generates the collector configuration, environment variables, and any errors encountered.
	// It takes all log pipelines and build options including cluster information and enrichment settings.
	Build(ctx context.Context, pipelines []telemetryv1beta1.LogPipeline, opts loggateway.BuildOptions) (*common.Config, common.EnvVars, error)
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
	Build(ctx context.Context, pipelines []telemetryv1beta1.LogPipeline, options logagent.BuildOptions) (*common.Config, common.EnvVars, error)
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

// PipelineLock manages exclusive access to pipeline resources to enforce maximum pipeline limits.
// It prevents exceeding the configured maximum number of active log pipelines in the cluster.
type PipelineLock interface {
	// TryAcquireLock attempts to acquire a lock for the given pipeline owner.
	// Returns an error if the maximum pipeline count is exceeded or if the lock cannot be acquired.
	// The lock ensures that only a limited number of pipelines can be active simultaneously.
	TryAcquireLock(ctx context.Context, owner metav1.Object) error

	// IsLockHolder checks if the given owner currently holds a lock.
	// Returns nil if the owner holds a lock, or an error if it does not.
	// This is used to determine if a pipeline is already registered and active.
	IsLockHolder(ctx context.Context, owner metav1.Object) error
}

// EndpointValidator validates log pipeline endpoint configurations.
// It checks if the endpoint is reachable, properly formatted, and compatible with the specified protocol.
type EndpointValidator interface {
	// Validate checks if the endpoint configuration is valid for the specified protocol.
	// It verifies the endpoint format, DNS resolution, and protocol compatibility.
	// Returns an error if the endpoint is invalid, unreachable, or incompatible with the protocol.
	Validate(ctx context.Context, params endpoint.EndpointValidationParams) error
}

// TLSCertValidator validates TLS certificate configurations for secure connections.
// It ensures certificates are valid, not expired, and properly formatted.
type TLSCertValidator interface {
	// Validate checks if the TLS certificate bundle is valid and not expired.
	// It verifies the certificate chain, expiration dates, and proper encoding.
	// Returns an error if the certificate is invalid, expired, or about to expire.
	Validate(ctx context.Context, config tlscert.TLSValidationParams) error
}

// SecretRefValidator validates secret references in LogPipeline resources.
// It ensures that all referenced Kubernetes secrets exist and are accessible.
type SecretRefValidator interface {
	// ValidateLogPipeline checks if all secret references in the pipeline exist and are accessible.
	// It verifies that secrets are present in the correct namespace and contain required keys.
	// Returns an error if any secret is missing, inaccessible, or malformed.
	ValidateLogPipeline(ctx context.Context, pipeline *telemetryv1beta1.LogPipeline) error
}

// TransformSpecValidator validates transform specifications in log pipeline configurations.
// It ensures that log transformations are correctly defined and syntactically valid.
type TransformSpecValidator interface {
	// Validate checks if the transform specifications are valid.
	// It verifies syntax, supported operations, and configuration completeness.
	// Returns an error if any transform is invalid or unsupported.
	Validate(transforms []telemetryv1beta1.TransformSpec) error
}

// FilterSpecValidator validates filter specifications in log pipeline configurations.
// It ensures that log filters are correctly defined and will work as expected.
type FilterSpecValidator interface {
	// Validate checks if the filter specifications are valid.
	// It verifies filter syntax, supported operations, and configuration completeness.
	// Returns an error if any filter is invalid or unsupported.
	Validate(filters []telemetryv1beta1.FilterSpec) error
}
