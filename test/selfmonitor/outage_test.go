package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/stdoutloggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOutage(t *testing.T) {
	tests := []struct {
		kind      string
		pipeline  func(includeNs string, backend *kitbackend.Backend) client.Object
		generator func(ns string) *appsv1.Deployment
		assert    func(t *testing.T)
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
				return stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(4000)).K8sObject()
			},
			assert: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelAgent)
				assert.LogPipelineConditionReasonsTransition(t, kindLogsOTelAgent, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentBufferFillingUp, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonAgentAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentAllDataDropped,
				})
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
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeLogs,
					telemetrygen.WithRate(800),
					telemetrygen.WithWorkers(5)).
					K8sObject()
			},
			assert: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelGateway)
				assert.LogPipelineConditionReasonsTransition(t, kindLogsOTelGateway, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
				})
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
				return stdoutloggen.NewDeployment(ns, stdoutloggen.WithRate(5000)).K8sObject()
			},
			assert: func(t *testing.T) {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, kindLogsFluentbit)
				assert.LogPipelineConditionReasonsTransition(t, kindLogsFluentbit, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentNoLogsDelivered, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonAgentAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentAllDataDropped,
				})
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
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeMetrics,
					telemetrygen.WithRate(10_000_000),
					telemetrygen.WithWorkers(50),
					telemetrygen.WithInterval("30s")).
					WithReplicas(2).
					K8sObject()
			},
			assert: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, kindMetrics)
				assert.MetricPipelineConditionReasonsTransition(t, kindMetrics, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewayBufferFillingUp, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeMetricComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
				})
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
				return telemetrygen.NewDeployment(ns, telemetrygen.SignalTypeTraces,
					telemetrygen.WithRate(80),
					telemetrygen.WithWorkers(10)).
					K8sObject()
			},
			assert: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, kindTraces)
				assert.TracePipelineConditionReasonsTransition(t, kindTraces, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewayBufferFillingUp, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonGatewayAllDataDropped, Status: metav1.ConditionFalse},
				})

				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeTraceComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewayAllDataDropped,
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			suite.RegisterTestCase(t, label(suite.LabelSelfMonitorOutage, tc.kind))

			var (
				uniquePrefix = unique.Prefix(tc.kind)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, signalType(tc.kind), kitbackend.WithReplicas(0)) // simulate outage
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

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			tc.assert(t)
		})
	}
}
