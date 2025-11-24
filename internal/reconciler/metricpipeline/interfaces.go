package metricpipeline

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricagent"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/metricgateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// AgentConfigBuilder builds OpenTelemetry Collector configuration for the metric agent from MetricPipeline resources.
// The agent runs as a DaemonSet and collects metrics from each node in the cluster.
type AgentConfigBuilder interface {
	// Build constructs the collector configuration and environment variables from the provided pipelines and build options.
	// Returns the complete agent configuration, environment variables, and any error encountered during the build process.
	Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, options metricagent.BuildOptions) (*common.Config, common.EnvVars, error)
}

// GatewayConfigBuilder builds OpenTelemetry Collector configuration for the metric gateway from MetricPipeline resources.
// The gateway receives metrics from agents and forwards them to external backends.
type GatewayConfigBuilder interface {
	// Build constructs the collector configuration and environment variables from the provided pipelines and build options.
	// Returns the complete gateway configuration, environment variables, and any error encountered during the build process.
	Build(ctx context.Context, pipelines []telemetryv1alpha1.MetricPipeline, options metricgateway.BuildOptions) (*common.Config, common.EnvVars, error)
}

// AgentApplierDeleter manages the lifecycle of metric agent Kubernetes resources.
// The agent runs as a DaemonSet on each cluster node to collect metrics.
type AgentApplierDeleter interface {
	// ApplyResources creates or updates the metric agent resources in the cluster.
	// This includes the DaemonSet, ConfigMap, ServiceAccount, and related resources.
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.AgentApplyOptions) error
	// DeleteResources removes the metric agent resources from the cluster.
	// This cleans up all agent-related Kubernetes resources.
	DeleteResources(ctx context.Context, c client.Client) error
}

// GatewayApplierDeleter manages the lifecycle of metric gateway Kubernetes resources.
// The gateway receives metrics from agents and forwards them to external backends.
type GatewayApplierDeleter interface {
	// ApplyResources creates or updates the metric gateway resources in the cluster.
	// This includes the Deployment, Service, ConfigMap, ServiceAccount, and related resources.
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	// DeleteResources removes the metric gateway resources from the cluster.
	// The isIstioActive flag determines whether Istio-specific resources should also be cleaned up.
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

// PipelineLock manages exclusive access to pipeline resources to enforce maximum pipeline limits.
// It prevents exceeding the configured maximum number of active metric pipelines in the cluster.
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

// PipelineSyncer synchronizes pipeline state and manages pipeline registration.
// It ensures pipelines are properly registered before they can be reconciled.
type PipelineSyncer interface {
	// TryAcquireLock attempts to register and acquire a lock for the given pipeline owner.
	// This is used during the initial reconciliation to register new pipelines.
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
}

// GatewayFlowHealthProber checks the health of metric data flow through the gateway for specific pipelines.
// It verifies that metrics are successfully flowing from the gateway to external backends.
type GatewayFlowHealthProber interface {
	// Probe performs a health check for the specified pipeline through the gateway.
	// Returns the probe result indicating whether metrics are flowing correctly.
	Probe(ctx context.Context, pipelineName string) (prober.OTelGatewayProbeResult, error)
}

// AgentFlowHealthProber checks the health of metric data flow through the agent for specific pipelines.
// It verifies that metrics are being collected and sent from agents to the gateway.
type AgentFlowHealthProber interface {
	// Probe performs a health check for the specified pipeline through the agent.
	// Returns the probe result indicating whether metrics are being collected and forwarded correctly.
	Probe(ctx context.Context, pipelineName string) (prober.OTelAgentProbeResult, error)
}

// OverridesHandler manages configuration overrides for telemetry components.
// It loads override configurations that can pause or modify telemetry behavior.
type OverridesHandler interface {
	// LoadOverrides loads the current override configuration from the cluster.
	// Returns the override configuration or an error if it cannot be loaded.
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

// IstioStatusChecker determines whether Istio service mesh is active in the cluster.
// This is used to conditionally configure Istio-specific resources and settings.
type IstioStatusChecker interface {
	// IsIstioActive returns true if Istio is currently active in the cluster.
	// This affects whether Istio-specific configurations (like PeerAuthentication) are applied.
	IsIstioActive(ctx context.Context) bool
}

// EndpointValidator validates metric pipeline endpoint configurations.
// It checks if the endpoint is reachable, properly formatted, and compatible with the specified protocol.
type EndpointValidator interface {
	// Validate checks if the endpoint configuration is valid for the specified protocol.
	// It verifies the endpoint format, DNS resolution, and protocol compatibility.
	// Returns an error if the endpoint is invalid, unreachable, or incompatible with the protocol.
	Validate(ctx context.Context, endpoint *telemetryv1alpha1.ValueType, protocol string) error
}

// TLSCertValidator validates TLS certificate configurations for secure connections.
// It ensures certificates are valid, not expired, and properly formatted.
type TLSCertValidator interface {
	// Validate checks if the TLS certificate bundle is valid and not expired.
	// It verifies the certificate chain, expiration dates, and proper encoding.
	// Returns an error if the certificate is invalid, expired, or about to expire.
	Validate(ctx context.Context, config tlscert.TLSBundle) error
}

// SecretRefValidator validates secret references in MetricPipeline resources.
// It ensures that all referenced Kubernetes secrets exist and are accessible.
type SecretRefValidator interface {
	// ValidateMetricPipeline checks if all secret references in the pipeline exist and are accessible.
	// It verifies that secrets are present in the correct namespace and contain required keys.
	// Returns an error if any secret is missing, inaccessible, or malformed.
	ValidateMetricPipeline(ctx context.Context, pipeline *telemetryv1alpha1.MetricPipeline) error
}

// TransformSpecValidator validates transform specifications in metric pipeline configurations.
// It ensures that metric transformations are correctly defined and syntactically valid.
type TransformSpecValidator interface {
	// Validate checks if the transform specifications are valid.
	// It verifies syntax, supported operations, and configuration completeness.
	// Returns an error if any transform is invalid or unsupported.
	Validate(transforms []telemetryv1alpha1.TransformSpec) error
}

// FilterSpecValidator validates filter specifications in metric pipeline configurations.
// It ensures that metric filters are correctly defined and will work as expected.
type FilterSpecValidator interface {
	// Validate checks if the filter specifications are valid.
	// It verifies filter syntax, supported operations, and configuration completeness.
	// Returns an error if any filter is invalid or unsupported.
	Validate(filters []telemetryv1alpha1.FilterSpec) error
}
