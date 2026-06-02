package commonstatus

import "time"

const (
	// RequeueDelayOnFlowHealthProbingFailure is the delay before requeuing when flow health probing fails
	// (e.g., self-monitor exists but is not ready yet). This allows time for the self-monitor to become healthy
	// before retrying the flow health probe.
	RequeueDelayOnFlowHealthProbingFailure = 30 * time.Second
)
