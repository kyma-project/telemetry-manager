package selfmonitor

import (
	"testing"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOutage(t *testing.T) {
	tests := []struct {
		prefix         string
		signalType     kitbackend.SignalType
		pipeline       func(includeNs string, backend *kitbackend.Backend) client.Object
		generator      func(ns string) client.Object
		resourcesReady func()
	}{
		{
			prefix:     logsOTelAgentPrefix,
			signalType: kitbackend.SignalTypeLogsOTel,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(logsOTelAgentPrefix).
					WithInput(testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) client.Object {
				return stdloggen.NewDeployment(ns).WithReplicas(3).K8sObject() // TODO: Any reason for 3 replicas?
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, logsOTelAgentPrefix)
			},
		},
		{
			prefix:     logsOTelGatewayPrefix,
			signalType: kitbackend.SignalTypeLogsOTel,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(logsOTelGatewayPrefix).
					WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) client.Object {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, logsOTelGatewayPrefix)
			},
		},
		{
			prefix:     logsFluentbitPrefix,
			signalType: kitbackend.SignalTypeLogsFluentBit,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(logsFluentbitPrefix).
					WithApplicationInput(true, testutils.ExtIncludeNamespaces(includeNs)).
					WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
					Build()
				return &p
			},
			generator: func(ns string) client.Object {
				return stdloggen.NewDeployment(ns).K8sObject()
			},
			resourcesReady: func() {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, logsFluentbitPrefix)
			},
		},
		{
			prefix:     metricsPrefix,
			signalType: kitbackend.SignalTypeMetrics,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(metricsPrefix).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeMetrics).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, metricsPrefix)
			},
		},
		{
			prefix:     tracesPrefix,
			signalType: kitbackend.SignalTypeTraces,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewTracePipelineBuilder().
					WithName(tracesPrefix).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) client.Object {
				return telemetrygen.NewPod(ns, telemetrygen.SignalTypeTraces).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, tracesPrefix)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelSelfMonitoringOutage)

			var (
				uniquePrefix = unique.Prefix(tc.prefix)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, tc.signalType, kitbackend.WithReplicas(0)) // simulate outage

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				tc.pipeline(genNs, backend),
				tc.generator(genNs),
			}
			resources = append(resources, backend.K8sObjects()...)

			t.Cleanup(func() {
				Expect(kitk8s.DeleteObjects(resources...)).To(Succeed())
			})
			Expect(kitk8s.CreateObjects(t, resources...)).To(Succeed())

			tc.resourcesReady()

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			// TODO: checkBufferFillingUp if signalType is metrics/traces
			// TODO: Stop sending data if signalType is metrics/traces
			// TODO: checkAllDataDropped
			checkMetricInstrumentation(t)
		})
	}
}

func checkMetricInstrumentation(t *testing.T) {
	t.Helper()

	// Pushing metrics to the metric gateway triggers an alert.
	// It makes the self-monitor call the webhook, which in turn increases the counter.
	assert.EmitsManagerMetrics(t,
		HaveName(Equal("controller_runtime_webhook_requests_total")),
		SatisfyAll(
			HaveLabels(HaveKeyWithValue("webhook", "/api/v2/alerts")),
			HaveMetricValue(BeNumerically(">", 0)),
		))

	assert.EmitsManagerMetrics(t,
		HaveName(Equal("telemetry_self_monitor_prober_requests_total")),
		HaveMetricValue(BeNumerically(">", 0)),
	)

}
