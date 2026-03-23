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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/faultbackend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

const bufferFillingUpRate = 60 * defaultRate

func TestBackpressure(t *testing.T) {
	tests := []struct {
		name           string
		component      string
		faultOpts      []faultbackend.Option
		generator      func(ns string) []client.Object
		expectedReason string
	}{
		{
			name:           "log-agent",
			component:      suite.LabelLogAgent,
			faultOpts:      faultNonRetryableErr(faultPercentageThirty),
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
		},
		{
			name:           "log-gateway",
			component:      suite.LabelLogGateway,
			faultOpts:      faultNonRetryableErr(faultPercentageThirty),
			expectedReason: conditions.ReasonSelfMonGatewaySomeDataDropped,
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
			generator:      stdoutLogGenerator(bufferFillingUpRate),
			expectedReason: conditions.ReasonSelfMonAgentBufferFillingUp,
		},
		{
			// HTTP 400 is non-retryable for Fluent Bit: data is dropped immediately without filling the queue → SomeDataDropped.
			name:           "fluent-bit-data-dropped",
			component:      suite.LabelFluentBit,
			faultOpts:      faultNonRetryableErr(faultPercentageThirty),
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
		},
		{
			name:           "metric-gateway",
			component:      suite.LabelMetricGateway,
			faultOpts:      faultNonRetryableErr(faultPercentageThirty),
			expectedReason: conditions.ReasonSelfMonGatewaySomeDataDropped,
		},
		{
			name:           "metric-agent",
			component:      suite.LabelMetricAgent,
			faultOpts:      faultNonRetryableErr(faultPercentageThirty),
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
		},
		{
			name:           "traces",
			component:      suite.LabelTraces,
			faultOpts:      faultNonRetryableErr(faultPercentageThirty),
			expectedReason: conditions.ReasonSelfMonGatewaySomeDataDropped,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{suite.LabelSelfMonitor, tc.component, suite.LabelBackpressure}

			var opts []kubeprep.Option
			if isFluentBit(tc.component) {
				opts = append(opts, kubeprep.WithOverrideFIPSMode(false), kubeprep.WithFluentBitHostPathCleanup())
			}

			suite.SetupTestWithOptions(t, labels, opts...)

			pipelineName := fmt.Sprintf("selfmonitor-%s", tc.name)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			fbOpts := tc.faultOpts
			if isFluentBit(tc.component) {
				fbOpts = append(fbOpts, faultbackend.WithFluentBitPort())
			}

			fb := faultbackend.New(backendNs, fbOpts...)
			pipeline := buildPipeline(tc.component, pipelineName, genNs, fb)

			gen := tc.generator
			if gen == nil {
				gen = defaultGenerator(tc.component)
			}

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, gen(genNs)...)
			resources = append(resources, fb.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			assertFlowDegraded(t, tc.component, pipelineName, flowHealthyThenDegraded(tc.expectedReason))
		})
	}
}
