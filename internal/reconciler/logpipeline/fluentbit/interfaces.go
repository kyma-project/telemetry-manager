package fluentbit

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/resources/fluentbit"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
)

// AgentConfigBuilder builds the Fluent Bit configuration from a set of reconcilable log pipelines.
// It generates the complete configuration that will be applied to the Fluent Bit agent.
type AgentConfigBuilder interface {
	// Build constructs the Fluent Bit configuration from the given pipelines and cluster name.
	// The cluster name is used for enriching logs with cluster-specific metadata.
	Build(ctx context.Context, reconcilablePipelines []telemetryv1alpha1.LogPipeline, clusterName string) (*builder.FluentBitConfig, error)
}

// AgentApplierDeleter manages the lifecycle of Fluent Bit agent resources in the cluster.
// It is responsible for both deploying and cleaning up the DaemonSet and related resources.
type AgentApplierDeleter interface {
	// ApplyResources creates or updates all Fluent Bit agent resources (DaemonSet, ConfigMaps, Services, etc.)
	// in the cluster based on the provided configuration and options.
	ApplyResources(ctx context.Context, c client.Client, opts fluentbit.AgentApplyOptions) error

	// DeleteResources removes all Fluent Bit agent resources from the cluster.
	// This is typically called when all log pipelines are deleted or non-reconcilable.
	DeleteResources(ctx context.Context, c client.Client) error
}

// IstioStatusChecker determines whether Istio service mesh is active in the cluster.
// This information is used to configure appropriate networking settings for Fluent Bit.
type IstioStatusChecker interface {
	// IsIstioActive checks if Istio is installed and active in the cluster.
	// When Istio is active, additional ports (like Envoy) may need to be configured.
	IsIstioActive(ctx context.Context) bool
}

// PipelineValidator validates the configuration of a LogPipeline resource.
// It performs various checks including endpoint validation, TLS certificate validation,
// secret reference validation, and resource locking.
type PipelineValidator interface {
	// Validate performs all validation checks on the given pipeline.
	// It returns an error if the pipeline configuration is invalid or if validation cannot be completed.
	Validate(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error
}

// ErrorToMessageConverter converts internal error types into user-friendly messages
// that can be displayed in the status conditions of a LogPipeline resource.
type ErrorToMessageConverter interface {
	// Convert translates an error into a human-readable message suitable for status conditions.
	// It handles various error types and provides appropriate context for debugging.
	Convert(err error) string
}

// FlowHealthProber checks the health of the log flow for a specific pipeline.
// It monitors metrics to detect issues like buffer filling, data drops, or delivery failures.
type FlowHealthProber interface {
	// Probe examines the health metrics of a specific pipeline and returns detailed results.
	// The results include information about buffer usage, data drops, and delivery status.
	Probe(ctx context.Context, pipelineName string) (prober.FluentBitProbeResult, error)
}

// AgentProber checks the readiness of the Fluent Bit agent DaemonSet.
// It verifies that pods are running and ready before considering the pipeline operational.
type AgentProber interface {
	// IsReady checks if the Fluent Bit DaemonSet identified by the given name is ready.
	// It returns an error if pods are not ready, in CrashLoopBackOff, OOMKilled, or other error states.
	IsReady(ctx context.Context, name types.NamespacedName) error
}
