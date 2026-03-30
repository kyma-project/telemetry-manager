package selfmonitor

import (
	"fmt"
	"testing"
	"time"

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

const bufferFillingUpRate = 60 * defaultRate

func TestBackpressure(t *testing.T) {
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
	}{
		{
			name:            "log-agent",
			component:       suite.LabelLogAgent,
			faultOpts:       faultNonRetryableErr(faultPercentageThirty),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonAgentSomeDataDropped),
		},
		{
			name:            "log-gateway",
			component:       suite.LabelLogGateway,
			faultOpts:       faultNonRetryableErr(faultPercentageThirty),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonGatewaySomeDataDropped),
		},
		{
			// HTTP 429 is retryable for Fluent Bit: the output plugin retries, so requests accumulate
			// in the Fluent Bit buffer → BufferFillingUp alert.
			//
			// 98% of requests receive HTTP 429 (retryable); Fluent Bit retries them but they never drain.
			// The remaining 2% succeed (HTTP 200), but those responses are delayed by 3 s, which limits
			// the successful drain throughput to ≈0.02 × (1/3 s) batches/s — far below the incoming
			// rate of bufferFillingUpRate (6000 lines/s). As a result the queue fills faster than it
			// empties, triggering the BufferFillingUp alert before SomeDataDropped.
			//
			// The delay is intentionally on the 200 path (successful drain), not the 429 path (retry):
			// slowing down the rare successful responses is sufficient to prevent the queue from clearing,
			// while keeping the retry loop fast enough to exercise the queue pressure code path.
			name:      "fluent-bit-buffer-filling-up",
			component: suite.LabelFluentBit,
			faultOpts: append(faultRetryableErr(faultPercentageNinetyEight),
				faultbackend.WithDelay(200, 3*time.Second),
			),
			generator:       stdoutLogGenerator(bufferFillingUpRate),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonAgentBufferFillingUp),
		},
		{
			// HTTP 400 is non-retryable for Fluent Bit: data is dropped immediately without filling the queue → SomeDataDropped.
			name:            "fluent-bit-data-dropped",
			component:       suite.LabelFluentBit,
			faultOpts:       faultNonRetryableErr(faultPercentageThirty),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonAgentSomeDataDropped),
		},
		{
			name:            "metric-gateway",
			component:       suite.LabelMetricGateway,
			faultOpts:       faultNonRetryableErr(faultPercentageThirty),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonGatewaySomeDataDropped),
		},
		{
			// Metric agent and gateway (using kyma stats receiver) both export data to the same backend.
			// Faulting the backend would affect both, masking the agent-specific alert.
			// An Istio VirtualService with a source-label match on telemetry-metric-agent pods
			// blocks only the agent→gateway leg, leaving the gateway's own backend traffic healthy.
			name:            "metric-agent",
			component:       suite.LabelMetricAgent,
			generator:       promMetricGeneratorHighLoad(),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonAgentSomeDataDropped),
			useIstio:        true,
		},
		{
			name:            "traces",
			component:       suite.LabelTraces,
			faultOpts:       faultNonRetryableErr(faultPercentageThirty),
			expectedReasons: degradedReasons(conditions.ReasonSelfMonGatewaySomeDataDropped),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{suite.LabelSelfMonitor, tc.component, suite.LabelBackpressure}

			opts := []kubeprep.Option{kubeprep.WithGatewayReplicas(1), kubeprep.WithSelfMonitorAPIServerEgress()}
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
				faultEnabler = newIstioFaultEnabler(
					"fault-injection", backendNs, backend.Name(),
					faultPercentageThirty,
					map[string]string{"app.kubernetes.io/name": "telemetry-metric-agent"},
				)
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

			// Wait for the pipeline to be healthy before enabling faults, so that
			// the self-monitor has a clean rate() baseline to detect the transition.
			assertComponentReady(t, tc.component)
			assertPipelineHealthy(t, tc.component, pipelineName)
			t.Log("Pipeline is healthy, enabling faults")

			faultEnabler.EnableFaults(t)

			assertFlowDegraded(t, tc.component, pipelineName, tc.expectedReasons)
		})
	}
}
