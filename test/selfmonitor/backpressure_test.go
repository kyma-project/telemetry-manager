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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestBackpressure(t *testing.T) {
	tests := []struct {
		name           string
		component      string
		backendOpts    []kitbackend.Option
		generator      func(ns string) []client.Object
		expectedReason string
	}{
		{
			name:           "log-agent",
			component:      suite.LabelLogAgent,
			backendOpts:    backendNonRetryableErr(faultPercentageHalf),
			generator:      stdoutLogGenerator(defaultRate),
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
		},
		{
			name:           "log-gateway",
			component:      suite.LabelLogGateway,
			backendOpts:    backendNonRetryableErr(faultPercentageHalf),
			generator:      otelGenerator(telemetrygen.SignalTypeLogs, telemetrygen.WithRate(defaultRate), telemetrygen.WithWorkers(1)),
			expectedReason: conditions.ReasonSelfMonGatewaySomeDataDropped,
		},
		{
			name:           "fluent-bit-buffer-filling-up",
			component:      suite.LabelFluentBit,
			backendOpts:    backendRetryableErr(faultPercentageNinetyFive),
			generator:      stdoutLogGenerator(60 * defaultRate),
			expectedReason: conditions.ReasonSelfMonAgentBufferFillingUp,
		},
		{
			name:           "fluent-bit-data-dropped",
			component:      suite.LabelFluentBit,
			backendOpts:    backendNonRetryableErr(faultPercentageHalf),
			generator:      stdoutLogGenerator(defaultRate),
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
		},
		{
			name:           "metric-gateway",
			component:      suite.LabelMetricGateway,
			backendOpts:    backendNonRetryableErr(faultPercentageHalf),
			generator:      otelGenerator(telemetrygen.SignalTypeMetrics, telemetrygen.WithRate(defaultRate), telemetrygen.WithWorkers(1)),
			expectedReason: conditions.ReasonSelfMonGatewaySomeDataDropped,
		},
		{
			name:           "metric-agent",
			component:      suite.LabelMetricAgent,
			backendOpts:    withMetricAgentSourceDrop(backendNonRetryableErr(faultPercentageHalf)),
			generator:      promMetricGeneratorHighLoad(),
			expectedReason: conditions.ReasonSelfMonAgentSomeDataDropped,
		},
		{
			name:           "traces",
			component:      suite.LabelTraces,
			backendOpts:    backendNonRetryableErr(faultPercentageHalf),
			generator:      otelGenerator(telemetrygen.SignalTypeTraces, telemetrygen.WithRate(defaultRate), telemetrygen.WithWorkers(1)),
			expectedReason: conditions.ReasonSelfMonGatewaySomeDataDropped,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			labels := []string{suite.LabelSelfMonitor, tc.component, suite.LabelBackpressure}

			opts := []kubeprep.Option{kubeprep.WithIstio()}
			if isFluentBit(tc.component) {
				opts = append(opts, kubeprep.WithOverrideFIPSMode(false))
			}

			suite.SetupTestWithOptions(t, labels, opts...)

			pipelineName := fmt.Sprintf("selfmonitor-%s", tc.name)

			var (
				uniquePrefix = unique.Prefix(tc.name)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			if isFluentBit(tc.component) {
				Expect(WaitForFluentBitDaemonSetGone(suite.Ctx, suite.K8sClient, TelemetryNamespace)).To(Succeed())
			}

			backend := kitbackend.New(backendNs, signalTypeForComponent(tc.component), tc.backendOpts...)
			pipeline := buildPipeline(tc.component, pipelineName, genNs, backend)

			resources := []client.Object{
				kitk8sobjects.NewNamespace(backendNs).K8sObject(),
				kitk8sobjects.NewNamespace(genNs).K8sObject(),
				pipeline,
			}
			resources = append(resources, tc.generator(genNs)...)
			resources = append(resources, backend.K8sObjects()...)
			if isFluentBit(tc.component) {
				resources = append(resources, FluentBitHostPathCleanupDaemonSet(TelemetryNamespace))
			}

			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			assert.SelfMonitorDebugOnFailure(t)
			assert.BackendReachable(t, backend)
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			assertFlowDegraded(t, tc.component, pipelineName, flowHealthyThenDegraded(tc.expectedReason))
		})
	}
}
