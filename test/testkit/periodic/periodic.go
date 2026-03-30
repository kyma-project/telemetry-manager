package periodic

import (
	"time"
)

const (
	// EventuallyTimeout is used when asserting an event with Eventually. Should be larger than periodic reconciliation interval.
	EventuallyTimeout = time.Second * 120

	// FlowHealthConditionTransitionTimeout is used when waiting for Log/Metric/Trace pipeline FlowHealthy to match a
	// self-monitor-driven reason (e.g. GatewayAllTelemetryDataDropped). Alert rules use a 5m rate() window and
	// typically `for: 1m`, then Prometheus must evaluate, the manager must reconcile, and scrapes must run — which
	// can exceed 10 minutes on loaded CI clusters.
	FlowHealthConditionTransitionTimeout = time.Minute * 10

	// ConsistentlyTimeout is used when asserting the absence of an event with Consistently.
	ConsistentlyTimeout = time.Second * 10

	// TelemetryEventuallyTimeout is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	// For example, to verify that a certain signal has provided resource attributes.
	TelemetryEventuallyTimeout = time.Second * 90

	// TelemetryConsistentlyTimeout is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	// For example, to verify that a certain signal *does not* have provided resource attributes.
	TelemetryConsistentlyTimeout = time.Second * 20

	// TelemetryConsistentlyScrapeTimeout is used to set the timeout equal to two scrape intervals.
	// So that we can consistently check for the presence/absence of a metric.
	TelemetryConsistentlyScrapeTimeout = time.Second * 60

	// DefaultInterval is the default interval duration used when no specialized interval is applicable.
	DefaultInterval = time.Millisecond * 500

	// SelfmonitorRateBaselineTimeout is used when waiting for Prometheus rate() queries to return
	// non-zero values before enabling faults. rate([5m]) needs at least 2 scrape samples (scrape
	// interval = 30s), but on loaded CI clusters the metric-agent scrape may take longer to settle.
	SelfmonitorRateBaselineTimeout = time.Minute * 5

	// SelfmonitorQueryInterval is the default interval duration used when checking for status changes related to Selfmonitor Alerts
	SelfmonitorQueryInterval = time.Second * 5

	// TelemetryInterval is used for asynchronous checks when polling Telemetry data from a mock backend via the export URL.
	TelemetryInterval = time.Second

	// SelfmonitorPodRestartWait is the time to wait after deleting a selfmonitor pod before retrying target discovery.
	// Allows the new pod's network namespace to initialize fully before Prometheus SD starts.
	SelfmonitorPodRestartWait = 10 * time.Second
)
