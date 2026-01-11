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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func FluentBitLogsFromContainerDelivered(t *testing.T, backend *kitbackend.Backend, expectedContainerName string, optionalDescription ...any) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		fluentbit.HaveFlatLogs(ContainElement(fluentbit.HaveContainerName(Equal(expectedContainerName)))),
		WithOptionalDescription(optionalDescription...),
	)
}

func FluentBitLogsFromContainerNotDelivered(t *testing.T, backend *kitbackend.Backend, expectedContainerName string, optionalDescription ...any) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		fluentbit.HaveFlatLogs(Not(ContainElement(fluentbit.HaveContainerName(Equal(expectedContainerName))))),
		WithOptionalDescription(optionalDescription...),
	)
}

func FluentBitLogsFromPodDelivered(t *testing.T, backend *kitbackend.Backend, expectedPodNamePrefix string, optionalDescription ...any) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		fluentbit.HaveFlatLogs(ContainElement(fluentbit.HavePodName(ContainSubstring(expectedPodNamePrefix)))),
		WithOptionalDescription(optionalDescription...),
	)
}

func FluentBitLogsFromNamespaceDelivered(t *testing.T, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		fluentbit.HaveFlatLogs(ContainElement(fluentbit.HaveNamespace(Equal(namespace)))),
		WithOptionalDescription(optionalDescription...),
	)
}

func FluentBitLogsFromNamespaceNotDelivered(t *testing.T, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		fluentbit.HaveFlatLogs(Not(ContainElement(fluentbit.HaveNamespace(Equal(namespace))))),
		WithOptionalDescription(optionalDescription...),
	)
}

func OTelLogsFromContainerDelivered(t *testing.T, backend *kitbackend.Backend, containerName string, optionalDescription ...any) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		HaveFlatLogs(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.container.name", containerName)))),
		WithOptionalDescription(optionalDescription...),
	)
}

func OTelLogsFromContainerNotDelivered(t *testing.T, backend *kitbackend.Backend, containerName string, optionalDescription ...any) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		HaveFlatLogs(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.container.name", containerName))))),
		WithOptionalDescription(optionalDescription...),
	)
}

func OTelLogsFromNamespaceDelivered(t *testing.T, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		HaveFlatLogs(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)))),
		WithOptionalDescription(optionalDescription...),
	)
}

func OTelLogsFromNamespaceNotDelivered(t *testing.T, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		HaveFlatLogs(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace))))),
		WithOptionalDescription(optionalDescription...),
	)
}

func OTelLogsFromPodNotDelivered(t *testing.T, backend *kitbackend.Backend, podNamePrefix string, optionalDescription ...any) {
	t.Helper()

	BackendDataConsistentlyMatches(
		t,
		backend,
		HaveFlatLogs(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(podNamePrefix)))))),
		WithOptionalDescription(optionalDescription...),
	)
}

func OTelLogsFromPodDelivered(t *testing.T, backend *kitbackend.Backend, podNamePrefix string, optionalDescription ...any) {
	t.Helper()

	BackendDataEventuallyMatches(
		t,
		backend,
		HaveFlatLogs(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.pod.name", ContainSubstring(podNamePrefix))))),
		WithOptionalDescription(optionalDescription...),
	)
}

//nolint:dupl //LogPipelineHealthy and MetricPipelineHealthy have similarities, but they are not the same
func FluentBitLogPipelineHealthy(t *testing.T, pipelineName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.LogPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())

		statusConditionHealthy(g, pipeline, conditions.TypeAgentHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeConfigurationGenerated)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineOtelHealthy and LogPipelineHealthy have similarities, but they are not the same
func OTelLogPipelineHealthy(t *testing.T, pipelineName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.LogPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())

		statusConditionHealthy(g, pipeline, conditions.TypeAgentHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeGatewayHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeConfigurationGenerated)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func statusConditionHealthy(g Gomega, pipeline telemetryv1beta1.LogPipeline, condType string) {
	condition := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(metav1.ConditionTrue), "Condition %s not healthy. Reason: %s. Message: %s", condType, condition.Reason, condition.Message)
}

//nolint:dupl // This provides a better readability for the test as we can test the TLS condition in a clear way
func LogPipelineHasCondition(t *testing.T, pipelineName string, expectedCond metav1.Condition) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.LogPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineConditionReasonsTransition,TracePipelineConditionReasonsTransition, MetricPipelineConditionReasonsTransition have similarities, but they are not the same
func LogPipelineConditionReasonsTransition(t *testing.T, pipelineName, condType string, expected []ReasonStatus) {
	t.Helper()

	var currCond *metav1.Condition

	for _, expected := range expected {
		// Wait for the current condition to match the expected condition
		Eventually(func(g Gomega) ReasonStatus {
			var pipeline telemetryv1beta1.LogPipeline

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

func LogPipelineUnsupportedMode(t *testing.T, pipelineName string, isUnsupportedMode bool) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.LogPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
		g.Expect(*pipeline.Status.UnsupportedMode).To(Equal(isUnsupportedMode))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl // TODO: Find a generic approach to merge this helper function with the other ones for the other telemetry types
func LogPipelineSelfMonitorIsHealthy(t *testing.T, k8sClient client.Client, pipelineName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1beta1.LogPipeline

		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
