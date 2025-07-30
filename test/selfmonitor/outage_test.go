package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/prometheus"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/floggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestOutage(t *testing.T) {
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
				return floggen.NewDeployment(ns).WithReplicas(2).K8sObject()
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
					telemetrygen.WithRate(10_000_000),
					telemetrygen.WithWorkers(50),
					telemetrygen.WithInterval("30s")).
					WithReplicas(2).
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
					telemetrygen.WithRate(80),
					telemetrygen.WithWorkers(10)).
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
			suite.RegisterTestCase(t, suite.LabelSelfMonitoringOutage)

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

			tc.resourcesReady()

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)
			assertBufferFillingUp(t, tc.kind)
			stopGenerator(t, generator)
			assertAllDataDropped(t, tc.kind)
			assertMetricInstrumentation(t)
		})
	}
}

// Waits for the flow to report a full buffer
func assertBufferFillingUp(t *testing.T, testKind string) {
	t.Helper()

	assert.MetricPipelineConditionReasonsTransition(t, testKind, conditions.TypeFlowHealthy, []assert.ReasonStatus{
		{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
		{Reason: bufferFillingUpConditionReason(testKind), Status: metav1.ConditionFalse},
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   componentsHealthyConditionType(testKind),
		Status: metav1.ConditionFalse,
		Reason: bufferFillingUpConditionReason(testKind),
	})
}

// Stops the generator.
// This is sometimes needed (for metrics and traces) to give the flow time to report a full buffer
func stopGenerator(t *testing.T, generator *appsv1.Deployment) {
	t.Helper()

	generator.Spec.Replicas = ptr.To(int32(0))
	err := suite.K8sClient.Update(suite.Ctx, generator)
	Expect(err).NotTo(HaveOccurred())
}

// Waits for the flow to gradually become unhealthy
func assertAllDataDropped(t *testing.T, testKind string) {
	t.Helper()

	assert.MetricPipelineConditionReasonsTransition(t, testKind, conditions.TypeFlowHealthy, []assert.ReasonStatus{
		{Reason: allDataDroppedConditionReason(testKind), Status: metav1.ConditionFalse},
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   componentsHealthyConditionType(testKind),
		Status: metav1.ConditionFalse,
		Reason: allDataDroppedConditionReason(testKind),
	})
}

func assertMetricInstrumentation(t *testing.T) {
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

func componentsHealthyConditionType(testKind string) string {
	switch signalType(testKind) {
	case kitbackend.SignalTypeLogsFluentBit, kitbackend.SignalTypeLogsOTel:
		return conditions.TypeLogComponentsHealthy
	case kitbackend.SignalTypeMetrics:
		return conditions.TypeMetricComponentsHealthy
	case kitbackend.SignalTypeTraces:
		return conditions.TypeTraceComponentsHealthy
	default:
		return ""
	}
}

func bufferFillingUpConditionReason(testKind string) string {
	switch testKind {
	case kindLogsOTelAgent, kindLogsFluentbit:
		return conditions.ReasonSelfMonAgentBufferFillingUp
	case kindLogsOTelGateway, kindMetrics, kindTraces:
		return conditions.ReasonSelfMonGatewayBufferFillingUp
	default:
		return ""
	}
}

func allDataDroppedConditionReason(testKind string) string {
	switch testKind {
	case kindLogsOTelAgent, kindLogsFluentbit:
		return conditions.ReasonSelfMonAgentAllDataDropped
	case kindLogsOTelGateway, kindMetrics, kindTraces:
		return conditions.ReasonSelfMonGatewayAllDataDropped
	default:
		return ""
	}
}
