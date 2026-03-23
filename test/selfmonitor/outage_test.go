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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/faultbackend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOutage(t *testing.T) {
	tests := []struct {
		name            string
		component       string
		faultOpts       []faultbackend.Option
		expectedReasons []assert.ReasonStatus
	}{
		{
			name:            "log-agent",
			component:       suite.LabelLogAgent,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:            "log-gateway",
			component:       suite.LabelLogGateway,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonGatewayAllDataDropped),
		},
		{
			// Connection close: Fluent Bit connects to port 9880 (HTTP/1.1, not HTTP/2) but the backend
			// immediately hijacks and closes the TCP connection, so no HTTP response is ever received.
			// Fluent Bit's HTTP output plugin counts this as no data delivered, which fires the
			// NoLogsDelivered alert in the self-monitor. This replaces the previous zero-replicas
			// approach (which caused ECONNREFUSED); both fault modes result in no successful delivery,
			// but WithDefaultClose() is more deterministic on clusters where port reuse can be delayed.
			name:            "fluent-bit-no-logs-delivered",
			component:       suite.LabelFluentBit,
			faultOpts:       []faultbackend.Option{faultbackend.WithDefaultClose()},
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentNoLogsDelivered),
		},
		{
			name:            "fluent-bit-all-data-dropped",
			component:       suite.LabelFluentBit,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:            "metric-gateway",
			component:       suite.LabelMetricGateway,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonGatewayAllDataDropped),
		},
		{
			name:            "metric-agent",
			component:       suite.LabelMetricAgent,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:            "traces",
			component:       suite.LabelTraces,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonGatewayAllDataDropped),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{suite.LabelSelfMonitor, tc.component, suite.LabelOutage}

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

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, defaultGenerator(tc.component)(genNs)...)
			resources = append(resources, fb.K8sObjects()...)

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			assertFlowDegraded(t, tc.component, pipelineName, tc.expectedReasons)
		})
	}
}
