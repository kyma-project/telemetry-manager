package selfmonitor

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

type testCaseOutage struct {
	kind                           string
	pipeline                       func(includeNs string, backend *kitbackend.Backend) client.Object
	generator                      func(ns string) *appsv1.Deployment
	resourcesReady                 func()
	conditionReasonsTransition     conditionReasonsTransitionFunc
	bufferFillingUpConditionReason string
	allDataDroppedConditionReason  string
	componentsHealthyConditionType string
}

func TestOutage(t *testing.T) {
	tests := []testCaseOutage{
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
			conditionReasonsTransition:     assert.LogPipelineConditionReasonsTransition,
			bufferFillingUpConditionReason: conditions.ReasonSelfMonAgentBufferFillingUp,
			allDataDroppedConditionReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			componentsHealthyConditionType: conditions.TypeLogComponentsHealthy,
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
			conditionReasonsTransition:     assert.LogPipelineConditionReasonsTransition,
			bufferFillingUpConditionReason: conditions.ReasonSelfMonGatewayBufferFillingUp,
			allDataDroppedConditionReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			componentsHealthyConditionType: conditions.TypeLogComponentsHealthy,
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
			conditionReasonsTransition:     assert.LogPipelineConditionReasonsTransition,
			bufferFillingUpConditionReason: conditions.ReasonSelfMonAgentBufferFillingUp,
			allDataDroppedConditionReason:  conditions.ReasonSelfMonAgentAllDataDropped,
			componentsHealthyConditionType: conditions.TypeLogComponentsHealthy,
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
			conditionReasonsTransition:     assert.MetricPipelineConditionReasonsTransition,
			bufferFillingUpConditionReason: conditions.ReasonSelfMonGatewayBufferFillingUp,
			allDataDroppedConditionReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			componentsHealthyConditionType: conditions.TypeMetricComponentsHealthy,
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
			conditionReasonsTransition:     assert.TracePipelineConditionReasonsTransition,
			bufferFillingUpConditionReason: conditions.ReasonSelfMonGatewayBufferFillingUp,
			allDataDroppedConditionReason:  conditions.ReasonSelfMonGatewayAllDataDropped,
			componentsHealthyConditionType: conditions.TypeTraceComponentsHealthy,
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

			tc.resourcesReady()

			assert.DeploymentReady(t, kitkyma.SelfMonitorName)

			if tc.kind == kindMetrics || tc.kind == kindTraces {
				assertBufferFillingUp(t, tc)
				stopGenerator(t, generator)
			}

			assertAllDataDropped(t, tc)
			assertMetricInstrumentation(t)
		})
	}
}

// Waits for the flow to report a full buffer
func assertBufferFillingUp(t *testing.T, tc testCaseOutage) {
	t.Helper()

	tc.conditionReasonsTransition(t, tc.kind, conditions.TypeFlowHealthy, []assert.ReasonStatus{
		{Reason: conditions.ReasonSelfMonFlowHealthy, Status: metav1.ConditionTrue},
		{Reason: tc.bufferFillingUpConditionReason, Status: metav1.ConditionFalse},
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   tc.componentsHealthyConditionType,
		Status: metav1.ConditionFalse,
		Reason: tc.bufferFillingUpConditionReason,
	})
}

// Stops the generator.
// This is sometimes needed (for metrics and traces) to give the flow time to report a full buffer
func stopGenerator(t *testing.T, generator *appsv1.Deployment) {
	t.Helper()

	var gen appsv1.Deployment

	genNamespacedName := types.NamespacedName{Namespace: generator.Namespace, Name: generator.Name}
	err := suite.K8sClient.Get(t.Context(), genNamespacedName, &gen)
	Expect(err).NotTo(HaveOccurred())

	generator.Spec.Replicas = ptr.To(int32(0))
	err = suite.K8sClient.Update(t.Context(), &gen)
	Expect(err).NotTo(HaveOccurred())
}

// Waits for the flow to gradually become unhealthy (i.e. all data dropped)
func assertAllDataDropped(t *testing.T, tc testCaseOutage) {
	t.Helper()

	tc.conditionReasonsTransition(t, tc.kind, conditions.TypeFlowHealthy, []assert.ReasonStatus{
		{Reason: tc.allDataDroppedConditionReason, Status: metav1.ConditionFalse},
	})

	assert.TelemetryHasState(t, operatorv1alpha1.StateWarning)
	assert.TelemetryHasCondition(t, suite.K8sClient, metav1.Condition{
		Type:   tc.componentsHealthyConditionType,
		Status: metav1.ConditionFalse,
		Reason: tc.allDataDroppedConditionReason,
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
