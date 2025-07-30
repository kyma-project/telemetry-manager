package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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
		kind                       string
		pipeline                   func(includeNs string, backend *kitbackend.Backend) client.Object
		generator                  func(ns string) *appsv1.Deployment
		resourcesReady             func()
		dataFromNamespaceDelivered func(ns string, backend *kitbackend.Backend)
		selfMonitorIsHealthy       func()
	}{
		{
			kind: kindLogsOTelAgent,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(kindLogsOTelAgent).
					WithInput(testutils.BuildLogPipelineApplicationInput(testutils.ExtIncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) *appsv1.Deployment {
				return stdloggen.NewDeployment(ns).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelAgent)
			},
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, kindLogsOTelAgent)
			},
		},
		{
			kind: kindLogsOTelGateway,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(kindLogsOTelGateway).
					WithInput(testutils.BuildLogPipelineOTLPInput(testutils.IncludeNamespaces(includeNs))).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) *appsv1.Deployment {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelGateway)
			},
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.OTelLogsFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, kindLogsOTelGateway)
			},
		},
		{
			kind: kindLogsFluentbit,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewLogPipelineBuilder().
					WithName(kindLogsFluentbit).
					WithApplicationInput(true, testutils.ExtIncludeNamespaces(includeNs)).
					WithHTTPOutput(testutils.HTTPHost(backend.Host()), testutils.HTTPPort(backend.Port())).
					Build()
				return &p
			},
			generator: func(ns string) *appsv1.Deployment {
				return stdloggen.NewDeployment(ns).K8sObject()
			},
			resourcesReady: func() {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, kindLogsFluentbit)
			},
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.FluentBitLogsFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.LogPipelineSelfMonitorIsHealthy(t, suite.K8sClient, kindLogsFluentbit)
			},
		},
		{
			kind: kindMetrics,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewMetricPipelineBuilder().
					WithName(kindMetrics).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) *appsv1.Deployment {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, kindMetrics)
			},
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.MetricsFromNamespaceDelivered(t, backend, ns, telemetrygen.MetricNames)
			},
			selfMonitorIsHealthy: func() {
				assert.MetricPipelineSelfMonitorIsHealthy(t, suite.K8sClient, kindMetrics)
			},
		},
		{
			kind: kindTraces,
			pipeline: func(includeNs string, backend *kitbackend.Backend) client.Object {
				p := testutils.NewTracePipelineBuilder().
					WithName(kindTraces).
					WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
					Build()
				return &p
			},
			generator: func(ns string) *appsv1.Deployment {
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeTraces).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, kindTraces)
			},
			dataFromNamespaceDelivered: func(ns string, backend *kitbackend.Backend) {
				assert.TracesFromNamespaceDelivered(t, backend, ns)
			},
			selfMonitorIsHealthy: func() {
				assert.TracePipelineSelfMonitorIsHealthy(t, suite.K8sClient, kindTraces)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelSelfMonitoringHealthy)

			var (
				uniquePrefix = unique.Prefix(tc.kind)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, signalType(tc.kind))
			pipeline := tc.pipeline(genNs, backend)
			generator := tc.generator(genNs)

			resources := []client.Object{
				kitk8s.NewNamespace(backendNs).K8sObject(),
				kitk8s.NewNamespace(genNs).K8sObject(),
				pipeline,
				generator,
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
