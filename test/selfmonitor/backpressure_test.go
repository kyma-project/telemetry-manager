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
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/floggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestBackpressure(t *testing.T) {
	tests := []struct {
		kind           string
		pipeline       func(includeNs string, backend *kitbackend.Backend) client.Object
		generator      func(ns string) *appsv1.Deployment
		resourcesReady func()
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
				return floggen.NewDeployment(ns).WithReplicas(3).K8sObject()
			},
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.DaemonSetReady(t, kitkyma.LogAgentName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelAgent)
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
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.LogGatewayName)
				assert.OTelLogPipelineHealthy(t, kindLogsOTelGateway)
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
				return floggen.NewDeployment(ns).WithReplicas(3).K8sObject()
			},
			resourcesReady: func() {
				assert.DaemonSetReady(t, kitkyma.FluentBitDaemonSetName)
				assert.FluentBitLogPipelineHealthy(t, kindLogsFluentbit)
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
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.MetricGatewayName)
				assert.MetricPipelineHealthy(t, kindMetrics)
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
			resourcesReady: func() {
				assert.DeploymentReady(t, kitkyma.TraceGatewayName)
				assert.TracePipelineHealthy(t, kindTraces)
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

			assert.BackendReachable(t, backend) // TODO(TeodorSAP): This might need to be removed, since backend is faulty in this test case
			tc.resourcesReady()

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			assertSomeDataDropped(t, tc.kind)
		})
	}
}

// Waits for the flow to gradually become unhealthy (i.e. some data dropped)
func assertSomeDataDropped(t *testing.T, testKind string) {
	t.Helper()

	assert.MetricPipelineConditionReasonsTransition(t, testKind, conditions.TypeFlowHealthy, []assert.ReasonStatus{
		{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
		{Reason: someDataDroppedConditionReason(testKind), Status: metav1.ConditionFalse},
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   componentsHealthyConditionType(testKind),
		Status: metav1.ConditionFalse,
		Reason: someDataDroppedConditionReason(testKind),
	})
}

func someDataDroppedConditionReason(testKind string) string {
	switch testKind {
	case kindLogsOTelAgent, kindLogsFluentbit:
		return conditions.ReasonSelfMonAgentSomeDataDropped
	case kindLogsOTelGateway, kindMetrics, kindTraces:
		return conditions.ReasonSelfMonGatewaySomeDataDropped
	default:
		return ""
	}
}
