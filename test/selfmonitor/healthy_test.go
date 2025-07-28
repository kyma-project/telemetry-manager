package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: Implement a table-driven test for the healthy self-monitoring scenario, covering all telemetry data types.
func TestHealthy(t *testing.T) {
	tests := []struct {
		name                       string
		signalType                 kitbackend.SignalType
		pipeline                   func(includeNs string, backend *kitbackend.Backend) client.Object
		generator                  func(ns string) client.Object
		resourcesReady             func()
		dataFromNamespaceDelivered func(ns string, backend *kitbackend.Backend)
		selfMonitorIsHealthy       func()
	}{
		{
			name: logsOTelAgentPrefix,
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
			name: logsOTelGatewayPrefix,
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
			name: logsFluentbitPrefix,
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			suite.RegisterTestCase(t, suite.LabelSelfMonitoringHealthy)

			var (
				uniquePrefix = unique.Prefix(tc.name)
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
