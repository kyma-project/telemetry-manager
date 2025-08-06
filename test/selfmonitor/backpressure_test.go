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

func TestBackpressure(t *testing.T) {
	tests := []struct {
		kind       string
		pipeline   func(includeNs string, backend *kitbackend.Backend) client.Object
		generator  func(ns string) *appsv1.Deployment
		assertions func(t *testing.T)
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
				return stdoutloggen.NewDeployment(ns,
					stdoutloggen.WithRate(800),
					stdoutloggen.WithWorkers(5),
				).K8sObject()
			},
			assertions: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelAgent)
				assert.LogPipelineConditionReasonsTransition(t, kindLogsOTelAgent, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentBufferFillingUp, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonAgentSomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentSomeDataDropped,
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
			assertions: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelGateway)
				assert.LogPipelineConditionReasonsTransition(t, kindLogsOTelGateway, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewaySomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewaySomeDataDropped,
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
				return stdoutloggen.NewDeployment(ns,
					stdoutloggen.WithRate(800),
					stdoutloggen.WithWorkers(5),
				).K8sObject()
			},
			assertions: func(t *testing.T) {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, kindLogsFluentbit)
				assert.LogPipelineConditionReasonsTransition(t, kindLogsFluentbit, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonAgentBufferFillingUp, Status: metav1.ConditionFalse},
					{Reason: conditions.ReasonSelfMonAgentSomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeLogComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonAgentSomeDataDropped,
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
					telemetrygen.WithRate(800),
					telemetrygen.WithWorkers(5)).
					K8sObject()
			},
			assertions: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, kindMetrics)
				assert.MetricPipelineConditionReasonsTransition(t, kindMetrics, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
					{Reason: conditions.ReasonSelfMonGatewaySomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeMetricComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewaySomeDataDropped,
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
					telemetrygen.WithRate(800),
					telemetrygen.WithWorkers(5)).
					K8sObject()
			},
			assertions: func(t *testing.T) {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, kindTraces)
				assert.TracePipelineConditionReasonsTransition(t, kindTraces, conditions.TypeFlowHealthy, []assert.ReasonStatus{
					{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},

					{Reason: conditions.ReasonSelfMonGatewaySomeDataDropped, Status: metav1.ConditionFalse},
				})
				assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
				assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
					Type:   conditions.TypeTraceComponentsHealthy,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonSelfMonGatewaySomeDataDropped,
				})
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			suite.RegisterTestCase(t, label(suite.LabelSelfMonitorBackpressure, tc.kind))

			var (
				uniquePrefix = unique.Prefix(tc.kind)
				backendNs    = uniquePrefix("backend")
				genNs        = uniquePrefix("gen")
			)

			backend := kitbackend.New(backendNs, signalType(tc.kind), kitbackend.WithAbortFaultInjection(85))
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
			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			tc.assertions(t)
		})
	}
}
