package selfmonitor

import (
	"testing"

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

func TestHealthy(t *testing.T) {
	tests := []struct {
		prefix                     string
		signalType                 kitbackend.SignalType
		pipeline                   func(includeNs string, backend *kitbackend.Backend) client.Object
		generator                  func(ns string) client.Object
		resourcesReady             func()
		dataFromNamespaceDelivered func(ns string, backend *kitbackend.Backend)
		selfMonitorIsHealthy       func()
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
				return stdloggen.NewDeployment(ns).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, logsOTelAgentPrefix)
			},
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, logsOTelAgentPrefix)
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
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, logsOTelGatewayPrefix)
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
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.FluentBitLogsFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, logsFluentbitPrefix)
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
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.MetricsFromNamespaceDelivered(t, backend, ns, telemetrygen.MetricNames)
			},
			selfMonitorIsHealthy: func() {
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, metricsPrefix)
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
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.TracesFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.TracePipelineSelfMonitorIsHealthy(t, suite.K8sClient, tracesPrefix)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelSelfMonitoringHealthy)

			var (
				uniquePrefix = unique.Prefix(tc.prefix)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, tc.signalType)

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

			assert.BackendReachable(t, backend)
			tc.resourcesReady()
			tc.dataFromNamespaceDelivered(genNs, backend)

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			tc.selfMonitorIsHealthy()
		})
	}
}
