package selfmonitor

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/faultbackend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOutage(t *testing.T) {
	tests := []struct {
		name            string
		component       string
		faultOpts       []faultbackend.Option
		generator       func(ns string) []client.Object
		expectedReasons []assert.ReasonStatus
		// useIstio indicates that this test case requires Istio fault injection via VirtualService
		// rather than the faultbackend. Used for metric-agent, where the VS selectively blocks only
		// agent→gateway traffic without affecting the gateway's own backend traffic.
		useIstio bool
		// skipHealthyBaseline skips waiting for the pipeline to be healthy before enabling faults.
		// Use this when the alert condition is satisfied even without a prior healthy baseline
		// (i.e. send_failed > 0 and sent == 0 from the very start), which avoids waiting for the
		// 5-minute rate window to decay after a healthy period.
		skipHealthyBaseline bool
	}{
		{
			name:                "log-agent",
			component:           suite.LabelLogAgent,
			faultOpts:           append(faultNonRetryableErr(faultPercentageAll), faultbackend.WithStartFaulted()),
			expectedReasons:     degradedReasons(conditions.ReasonSelfMonAgentAllDataDropped),
			skipHealthyBaseline: true,
		},
		{
			name:                "log-gateway",
			component:           suite.LabelLogGateway,
			faultOpts:           append(faultNonRetryableErr(faultPercentageAll), faultbackend.WithStartFaulted()),
			expectedReasons:     degradedReasons(conditions.ReasonSelfMonGatewayAllDataDropped),
			skipHealthyBaseline: true,
		},
		{
			// Connection close: Fluent Bit connects to port 9880 (HTTP/1.1, not HTTP/2) but the backend
			// immediately hijacks and closes the TCP connection, so no HTTP response is ever received.
			// Fluent Bit's HTTP output plugin counts this as no data delivered, which fires the
			// NoLogsDelivered alert in the self-monitor. This replaces the previous zero-replicas
			// approach (which caused ECONNREFUSED); both fault modes result in no successful delivery,
			// but WithDefaultClose() is more deterministic on clusters where port reuse can be delayed.
			name:                "fluent-bit-no-logs-delivered",
			component:           suite.LabelFluentBit,
			faultOpts:           []faultbackend.Option{faultbackend.WithDefaultClose(), faultbackend.WithStartFaulted()},
			expectedReasons:     degradedReasons(conditions.ReasonSelfMonAgentNoLogsDelivered),
			skipHealthyBaseline: true,
		},
		{
			name:            "fluent-bit-all-data-dropped",
			component:       suite.LabelFluentBit,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:                "metric-gateway",
			component:           suite.LabelMetricGateway,
			faultOpts:           append(faultNonRetryableErr(faultPercentageAll), faultbackend.WithStartFaulted()),
			expectedReasons:     degradedReasons(conditions.ReasonSelfMonGatewayAllDataDropped),
			skipHealthyBaseline: true,
		},
		{
			// Metric agent and gateway (using kyma stats receiver) both export data to the same backend.
			// Faulting the backend would affect both, masking the agent-specific alert.
			// An Istio VirtualService with a source-label match on telemetry-metric-agent pods
			// blocks all requests from the agent→gateway leg, leaving the gateway's own backend traffic healthy.
			// sourceLabels is an Istio selector (not a runtime match): the VS config is only
			// pushed to sidecars of pods matching the label, so the gateway never sees it.
			name:                "metric-agent",
			component:           suite.LabelMetricAgent,
			generator:           promMetricGeneratorHighLoad(),
			expectedReasons:     degradedReasons(conditions.ReasonSelfMonAgentAllDataDropped),
			useIstio:            true,
			skipHealthyBaseline: true,
		},
		{
			name:                "traces",
			component:           suite.LabelTraces,
			faultOpts:           append(faultNonRetryableErr(faultPercentageAll), faultbackend.WithStartFaulted()),
			expectedReasons:     degradedReasons(conditions.ReasonSelfMonGatewayAllDataDropped),
			skipHealthyBaseline: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{suite.LabelSelfMonitor, tc.component, suite.LabelOutage}

			opts := []kubeprep.Option{kubeprep.WithGatewayReplicas(1)}
			if tc.useIstio {
				opts = append(opts, kubeprep.WithIstio())
			}

			if isFluentBit(tc.component) {
				opts = append(opts, kubeprep.WithOverrideFIPSMode(false), kubeprep.WithFluentBitHostPathCleanup())
			}

			suite.SetupTestWithOptions(t, labels, opts...)
			enableDebugLogging(t)

			pipelineName := fmt.Sprintf("selfmonitor-%s", tc.name)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			gen := tc.generator
			if gen == nil {
				gen = defaultGenerator(tc.component)
			}

			var (
				pipeline     client.Object
				resources    []client.Object
				faultEnabler FaultEnabler
			)

			resources = append(resources,
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
			)

			if tc.useIstio {
				// Use a regular backend and inject faults via a VirtualService that targets only
				// telemetry-metric-agent pods, leaving the gateway's backend traffic unaffected.
				// sourceLabels is an Istio selector (not a runtime match): the VS config is only
				// pushed to sidecars of pods matching the label, so the gateway never sees it.
				backend := kitbackend.New(backendNs, signalTypeForComponent(tc.component))
				pipeline = buildPipeline(tc.component, pipelineName, genNs, backend)
				resources = append(resources, backend.K8sObjects()...)

				ife := newIstioFaultEnabler(
					"fault-injection", backendNs, backend.Name(),
					faultPercentageAll,
					map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"},
				)
				if tc.skipHealthyBaseline {
					resources = append(resources, ife.K8sObjects()...)
				} else {
					faultEnabler = ife
				}
			} else {
				fbOpts := tc.faultOpts
				if isFluentBit(tc.component) {
					fbOpts = append(fbOpts, faultbackend.WithFluentBitPort())
				}

				fb := faultbackend.New(backendNs, fbOpts...)
				pipeline = buildPipeline(tc.component, pipelineName, genNs, fb)
				resources = append(resources, fb.K8sObjects()...)
				faultEnabler = fb
			}

			resources = append(resources, pipeline)
			resources = append(resources, gen(genNs)...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())
			logDiagnosticsOnFailure(t, tc.component)

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			assertSelfMonitorHasActiveTargets(t)
			assertComponentReady(t, tc.component)

			if tc.skipHealthyBaseline {
				// Faults are active from boot (WithStartFaulted): no EnableFaults call needed,
				// and no healthy baseline is required. The alert fires as soon as send_failed > 0
				// and sent == 0, which is the case from the start.
				t.Log("Faults active from boot, skipping healthy baseline")
			} else {
				// Wait for the pipeline to be healthy before enabling faults, so that
				// the self-monitor has a clean rate() baseline to detect the transition.
				assertPipelineHealthy(t, tc.component, pipelineName)
				t.Log("Pipeline is healthy, enabling faults")
				faultEnabler.EnableFaults(t)
			}

			assertFlowDegraded(t, tc.component, pipelineName, tc.expectedReasons)
		})
	}
}
