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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TestOutage checks FlowHealthy degrades when the pipeline cannot deliver telemetry.
//
// OTel-based rows (log-agent, log-gateway, metric-gateway, metric-agent, traces) use a FaultBackend
// that returns HTTP 400 (non-retryable) for 100% of requests. Fluent Bit has two rows:
// no-endpoints (backendNoEndpoints → NoLogsDelivered then AllDataDropped) and HTTP 400 abort (all dropped).
func TestOutage(t *testing.T) {
	tests := []struct {
		name            string
		component       string
		faultOpts       []faultbackend.Option
		backendOpts     []kitbackend.Option
		useFaultBackend bool
		generator       func(ns string) []client.Object
		expectedReasons []assert.ReasonStatus
	}{
		{
			name:            "log-agent",
			component:       suite.LabelLogAgent,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			useFaultBackend: true,
			generator:       stdoutLogGenerator(4000),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:            "log-gateway",
			component:       suite.LabelLogGateway,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			useFaultBackend: true,
			generator:       otelGenerator(telemetrygen.SignalTypeLogs, telemetrygen.WithRate(800), telemetrygen.WithWorkers(5)),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonGatewayAllDataDropped),
		},
		{
			// No backend endpoints: Fluent Bit reads logs but cannot complete output; FlowHealthy moves through
			// NoLogsDelivered to AllDataDropped.
			name:            "fluent-bit-no-logs-delivered",
			component:       suite.LabelFluentBit,
			backendOpts:     backendNoEndpoints(),
			useFaultBackend: false,
			generator:       stdoutLogGenerator(5000),
			expectedReasons: flowHealthyThenDegraded(
				conditions.ReasonSelfMonAgentNoLogsDelivered,
				conditions.ReasonSelfMonAgentAllDataDropped,
			),
		},
		{
			name:            "fluent-bit-all-data-dropped",
			component:       suite.LabelFluentBit,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			useFaultBackend: true,
			generator:       stdoutLogGenerator(defaultRate),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:            "metric-gateway",
			component:       suite.LabelMetricGateway,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			useFaultBackend: true,
			generator: func(ns string) []client.Object {
				return []client.Object{
					telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics,
						telemetrygen.WithRate(10_000_000),
						telemetrygen.WithWorkers(50),
						telemetrygen.WithInterval("30s"),
					).WithReplicas(2).K8sObject(),
				}
			},
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonGatewayAllDataDropped),
		},
		{
			name:            "metric-agent",
			component:       suite.LabelMetricAgent,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			useFaultBackend: true,
			generator:       promMetricGeneratorHighLoad(),
			expectedReasons: flowHealthyThenDegraded(conditions.ReasonSelfMonAgentAllDataDropped),
		},
		{
			name:            "traces",
			component:       suite.LabelTraces,
			faultOpts:       faultNonRetryableErr(faultPercentageAll),
			useFaultBackend: true,
			generator:       otelGenerator(telemetrygen.SignalTypeTraces, telemetrygen.WithRate(80), telemetrygen.WithWorkers(10)),
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

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
			}

			if tc.useFaultBackend {
				fbOpts := tc.faultOpts
				if isFluentBit(tc.component) {
					fbOpts = append(fbOpts, faultbackend.WithFluentBitPort())
				}

				fb := faultbackend.New(backendNs, fbOpts...)
				pipeline := buildPipeline(tc.component, pipelineName, genNs, fb)
				resources = append(resources, pipeline)
				resources = append(resources, tc.generator(genNs)...)
				resources = append(resources, fb.K8sObjects()...)
			} else {
				backend := kitbackend.New(backendNs, signalTypeForComponent(tc.component), tc.backendOpts...)
				pipeline := buildPipeline(tc.component, pipelineName, genNs, backend)
				resources = append(resources, pipeline)
				resources = append(resources, tc.generator(genNs)...)
				resources = append(resources, backend.K8sObjects()...)
			}

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.SelfMonitorDebugOnFailure(t)
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			assertFlowDegraded(t, tc.component, pipelineName, tc.expectedReasons)
		})
	}
}
