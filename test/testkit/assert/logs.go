package assert

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/log/fluentbit"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func FluentBitLogsFromContainerDelivered(ctx context.Context, backend *kitbackend.Backend, expectedContainerName string, optionalDescription ...any) {
	BackendDataEventuallyMatches(
		ctx,
		backend,
		fluentbit.HaveFlatLogs(ContainElement(fluentbit.HaveContainerName(Equal(expectedContainerName)))),
		optionalDescription...,
	)
}

func FluentBitLogsFromContainerNotDelivered(ctx context.Context, backend *kitbackend.Backend, expectedContainerName string, optionalDescription ...any) {
	BackendDataConsistentlyMatches(
		ctx,
		backend,
		fluentbit.HaveFlatLogs(Not(ContainElement(fluentbit.HaveContainerName(Equal(expectedContainerName))))),
		optionalDescription...,
	)
}

func FluentBitLogsFromPodDelivered(ctx context.Context, backend *kitbackend.Backend, expectedPodNamePrefix string, optionalDescription ...any) {
	BackendDataEventuallyMatches(
		ctx,
		backend,
		fluentbit.HaveFlatLogs(ContainElement(fluentbit.HavePodName(ContainSubstring(expectedPodNamePrefix)))),
		optionalDescription...,
	)
}

func FluentBitLogsFromNamespaceDelivered(ctx context.Context, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	BackendDataEventuallyMatches(
		ctx,
		backend,
		fluentbit.HaveFlatLogs(ContainElement(fluentbit.HaveNamespace(Equal(namespace)))),
		optionalDescription...,
	)
}

func FluentBitLogsFromNamespaceNotDelivered(ctx context.Context, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	BackendDataConsistentlyMatches(
		ctx,
		backend,
		fluentbit.HaveFlatLogs(Not(ContainElement(fluentbit.HaveNamespace(Equal(namespace))))),
		optionalDescription...,
	)
}

func OTelLogsFromContainerDelivered(ctx context.Context, backend *kitbackend.Backend, containerName string, optionalDescription ...any) {
	BackendDataEventuallyMatches(
		ctx,
		backend,
		HaveFlatLogs(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.container.name", containerName)))),
		optionalDescription...,
	)
}

func OTelLogsFromContainerNotDelivered(ctx context.Context, backend *kitbackend.Backend, containerName string, optionalDescription ...any) {
	BackendDataConsistentlyMatches(
		ctx,
		backend,
		HaveFlatLogs(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.container.name", containerName))))),
		optionalDescription...,
	)
}

func OTelLogsFromNamespaceDelivered(ctx context.Context, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	BackendDataEventuallyMatches(
		ctx,
		backend,
		HaveFlatLogs(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)))),
		optionalDescription...,
	)
}

func OTelLogsFromNamespaceNotDelivered(ctx context.Context, backend *kitbackend.Backend, namespace string, optionalDescription ...any) {
	BackendDataConsistentlyMatches(
		ctx,
		backend,
		HaveFlatLogs(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace))))),
		optionalDescription...,
	)
}

//nolint:dupl //LogPipelineHealthy and MetricPipelineHealthy have similarities, but they are not the same
func FluentBitLogPipelineHealthy(ctx context.Context, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(ctx, key, &pipeline)).To(Succeed())

		statusConditionHealthy(g, pipeline, conditions.TypeAgentHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeConfigurationGenerated)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineOtelHealthy and LogPipelineHealthy have similarities, but they are not the same
func OTelLogPipelineHealthy(ctx context.Context, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(ctx, key, &pipeline)).To(Succeed())

		statusConditionHealthy(g, pipeline, conditions.TypeAgentHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeGatewayHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeConfigurationGenerated)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func statusConditionHealthy(g Gomega, pipeline telemetryv1alpha1.LogPipeline, condType string) {
	condition := meta.FindStatusCondition(pipeline.Status.Conditions, condType)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(condition.Status).To(Equal(metav1.ConditionTrue), "Condition %s not healthy. Reason: %s. Message: %s", condType, condition.Reason, condition.Message)
}

//nolint:dupl // This provides a better readability for the test as we can test the TLS condition in a clear way
func LogPipelineHasCondition(ctx context.Context, pipelineName string, expectedCond metav1.Condition) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineConditionReasonsTransition,TracePipelineConditionReasonsTransition, MetricPipelineConditionReasonsTransition have similarities, but they are not the same
func LogPipelineConditionReasonsTransition(ctx context.Context, pipelineName, condType string, expected []ReasonStatus) {
	var currCond *metav1.Condition

	for _, expected := range expected {
		// Wait for the current condition to match the expected condition
		Eventually(func(g Gomega) ReasonStatus {
			var pipeline telemetryv1alpha1.LogPipeline
			key := types.NamespacedName{Name: pipelineName}
			err := suite.K8sClient.Get(ctx, key, &pipeline)
			g.Expect(err).To(Succeed())
			currCond = meta.FindStatusCondition(pipeline.Status.Conditions, condType)
			if currCond == nil {
				return ReasonStatus{}
			}

			return ReasonStatus{Reason: currCond.Reason, Status: currCond.Status}
		}, 10*time.Minute, periodic.DefaultInterval).Should(Equal(expected), "expected reason %s[%s] of type %s not reached", expected.Reason, expected.Status, condType)

		fmt.Fprintf(GinkgoWriter, "Transitioned to [%s]%s\n", currCond.Status, currCond.Reason)
	}
}

func LogPipelineUnsupportedMode(ctx context.Context, pipelineName string, isUnsupportedMode bool) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(suite.K8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(*pipeline.Status.UnsupportedMode).To(Equal(isUnsupportedMode))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
