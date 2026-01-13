package tracepipeline

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/tracegateway"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/resources/otelcollector"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	"github.com/kyma-project/telemetry-manager/internal/validators/endpoint"
	"github.com/kyma-project/telemetry-manager/internal/validators/tlscert"
)

// GatewayConfigBuilder builds OpenTelemetry Collector configuration for the trace gateway from TracePipeline resources.
type GatewayConfigBuilder interface {
	// Build constructs the collector configuration and environment variables from the provided pipelines and build options.
	Build(ctx context.Context, pipelines []telemetryv1beta1.TracePipeline, opts tracegateway.BuildOptions) (*common.Config, common.EnvVars, error)
}

// GatewayApplierDeleter manages the lifecycle of trace gateway Kubernetes resources.
type GatewayApplierDeleter interface {
	// ApplyResources creates or updates the trace gateway resources in the cluster.
	ApplyResources(ctx context.Context, c client.Client, opts otelcollector.GatewayApplyOptions) error
	// DeleteResources removes the trace gateway resources from the cluster.
	DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error
}

// PipelineLock manages exclusive access to pipeline resources to enforce maximum pipeline limits.
type PipelineLock interface {
	// TryAcquireLock attempts to acquire a lock for the given pipeline owner.
	// Returns an error if the maximum pipeline count is exceeded or if the lock cannot be acquired.
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
	// IsLockHolder checks if the given owner currently holds a lock.
	IsLockHolder(ctx context.Context, owner metav1.Object) error
}

// PipelineSyncer synchronizes pipeline state and manages pipeline registration.
type PipelineSyncer interface {
	// TryAcquireLock attempts to register and acquire a lock for the given pipeline owner.
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
}

// FlowHealthProber checks the health of trace data flow through the gateway for specific pipelines.
type FlowHealthProber interface {
	// Probe performs a health check for the specified pipeline and returns the probe result.
	Probe(ctx context.Context, pipelineName string) (prober.OTelGatewayProbeResult, error)
}

// OverridesHandler manages configuration overrides for telemetry components.
type OverridesHandler interface {
	// LoadOverrides loads the current override configuration from the cluster.
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

// IstioStatusChecker determines whether Istio service mesh is active in the cluster.
type IstioStatusChecker interface {
	// IsIstioActive returns true if Istio is currently active in the cluster.
	IsIstioActive(ctx context.Context) bool
}

// EndpointValidator validates trace pipeline endpoint configurations.
type EndpointValidator interface {
	// Validate checks if the endpoint configuration is valid for the specified protocol.
	Validate(ctx context.Context, params endpoint.EndpointValidationParams) error
}

// SecretRefValidator validates secret references in TracePipeline resources.
type SecretRefValidator interface {
	// ValidateTracePipeline checks if all secret references in the pipeline exist and are accessible.
	ValidateTracePipeline(ctx context.Context, pipeline *telemetryv1beta1.TracePipeline) error
}

// TLSCertValidator validates TLS certificate configurations.
type TLSCertValidator interface {
	// Validate checks if the TLS certificate bundle is valid and not expired.
	Validate(ctx context.Context, config tlscert.TLSValidationParams) error
}

// TransformSpecValidator validates transform specifications in pipeline configurations.
type TransformSpecValidator interface {
	// Validate checks if the transform specifications are valid.
	Validate(transforms []telemetryv1beta1.TransformSpec) error
}

// FilterSpecValidator validates filter specifications in pipeline configurations.
type FilterSpecValidator interface {
	// Validate checks if the filter specifications are valid.
	Validate(filters []telemetryv1beta1.FilterSpec) error
}
