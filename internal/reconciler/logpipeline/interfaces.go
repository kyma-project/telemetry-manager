package logpipeline

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/overrides"
	"github.com/kyma-project/telemetry-manager/internal/selfmonitor/prober"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

// FlowHealthProber provides health probing capabilities for Fluent Bit data flows.
// It checks the health of log pipelines by examining metrics and indicators from the Fluent Bit agent.
type FlowHealthProber interface {
	// Probe checks the health of a specific pipeline by name and returns detailed probe results.
	// The probe results include information about buffer status, data delivery, and any detected issues.
	Probe(ctx context.Context, pipelineName string) (prober.FluentBitProbeResult, error)
}

// LogPipelineReconciler reconciles a LogPipeline resource based on its output type.
// Different reconcilers handle different output modes (e.g., Fluent Bit custom output vs OTel collector).
type LogPipelineReconciler interface {
	// Reconcile processes a LogPipeline and ensures the desired state is achieved.
	// This includes deploying necessary resources, updating configurations, and managing the pipeline lifecycle.
	Reconcile(ctx context.Context, pipeline *telemetryv1alpha1.LogPipeline) error

	// SupportedOutput returns the output mode that this reconciler supports (e.g., FluentBit or OTel).
	SupportedOutput() logpipelineutils.Mode
}

// OverridesHandler manages loading and processing of configuration overrides.
// Overrides allow administrators to customize the behavior of the telemetry system beyond default settings.
type OverridesHandler interface {
	// LoadOverrides retrieves the current override configuration from the cluster.
	// Returns an error if the overrides cannot be loaded or parsed.
	LoadOverrides(ctx context.Context) (*overrides.Config, error)
}

// PipelineSyncer manages synchronization and locking for pipeline resources.
// It ensures that only one controller instance can reconcile a pipeline at a time to prevent conflicts.
type PipelineSyncer interface {
	// TryAcquireLock attempts to acquire a lock for the given pipeline owner.
	// Returns an error if the lock cannot be acquired or if another controller holds the lock.
	TryAcquireLock(ctx context.Context, owner metav1.Object) error
}
