package stubs

import (
	"context"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	logpipelineutils "github.com/kyma-project/telemetry-manager/internal/utils/logpipeline"
)

// ReconcilerStub is a stub implementation of LogPipelineReconciler for testing.
type ReconcilerStub struct {
	OutputType logpipelineutils.Mode
	Result     error
}

func (r *ReconcilerStub) Reconcile(_ context.Context, _ *telemetryv1beta1.LogPipeline) error {
	return r.Result
}

func (r *ReconcilerStub) SupportedOutput() logpipelineutils.Mode {
	return r.OutputType
}
