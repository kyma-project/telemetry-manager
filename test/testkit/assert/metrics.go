package assert

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func MetricsFromNamespaceDelivered(t *testing.T, backend *kitbackend.Backend, namespace string, metricNames []string) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		HaveFlatMetrics(ContainElement(SatisfyAll(
			HaveName(BeElementOf(metricNames)),
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)),
		))),
	)
}

func MetricsFromNamespaceNotDelivered(t *testing.T, backend *kitbackend.Backend, namespace string) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		HaveFlatMetrics(
			Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)))),
		),
	)
}

func MetricsWithScopeAndNamespaceNotDelivered(t *testing.T, backend *kitbackend.Backend, scope, namespace string, optionalDescription ...any) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		HaveFlatMetrics(Not(ContainElement(SatisfyAll(
			HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)),
			HaveResourceAttributes(HaveKeyWithValue("service", scope)),
		)))),
		WithOptionalDescription(optionalDescription...),
	)
}

func MetricPipelineHealthy(t *testing.T, pipelineName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.MetricPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())

		agentHealthy := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeAgentHealthy)
		g.Expect(agentHealthy).NotTo(BeNil())
		g.Expect(agentHealthy.Status).To(Equal(metav1.ConditionTrue), "Agent not healthy. Reason: %s. Message: %s", agentHealthy.Reason, agentHealthy.Message)

		gatewayHealthy := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeGatewayHealthy)
		g.Expect(gatewayHealthy).NotTo(BeNil())
		g.Expect(gatewayHealthy.Status).To(Equal(metav1.ConditionTrue), "Gateway not healthy. Reason: %s. Message: %s", gatewayHealthy.Reason, gatewayHealthy.Message)

		configGenerated := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		g.Expect(configGenerated).NotTo(BeNil())
		g.Expect(configGenerated.Status).To(Equal(metav1.ConditionTrue), "Configuration not generated. Reason: %s. Message: %s", configGenerated.Reason, configGenerated.Message)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func MetricPipelineHasCondition(t *testing.T, pipelineName string, expectedCond metav1.Condition) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.MetricPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

type ReasonStatus struct {
	Reason string
	Status metav1.ConditionStatus
}

//nolint:dupl //LogPipelineConditionReasonsTransition,TracePipelineConditionReasonsTransition, MetricPipelineConditionReasonsTransition have similarities, but they are not the same
func MetricPipelineConditionReasonsTransition(t *testing.T, pipelineName, condType string, expected []ReasonStatus) {
	t.Helper()

	var currCond *metav1.Condition

	for _, expected := range expected {
		// Wait for the current condition to match the expected condition
		Eventually(func(g Gomega) ReasonStatus {
			var pipeline telemetryv1beta1.MetricPipeline

			key := types.NamespacedName{Name: pipelineName}
			err := suite.K8sClient.Get(t.Context(), key, &pipeline)
			g.Expect(err).To(Succeed())

			currCond = meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			if currCond == nil {
				return ReasonStatus{}
			}

			return ReasonStatus{Reason: currCond.Reason, Status: currCond.Status}
		}, 10*time.Minute, periodic.DefaultInterval).Should(Equal(expected), "expected reason %s[%s] of type %s not reached", expected.Reason, expected.Status, condType)

		t.Logf("Transitioned to [%s]%s\n", currCond.Status, currCond.Reason)
	}
}

//nolint:dupl // TODO: Find a generic approach to merge this helper function with the other ones for the other telemetry types
func MetricPipelineSelfMonitorIsHealthy(t *testing.T, k8sClient client.Client, pipelineName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.MetricPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
