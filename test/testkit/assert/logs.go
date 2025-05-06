package assert

import (
	"context"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func LogsDelivered(proxyClient *apiserverproxy.Client, expectedPodNamePrefix string, backendExportURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatFluentBitLogs(ContainElement(
				HavePodName(ContainSubstring(expectedPodNamePrefix))),
			)))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func FBLogsFromNamespaceDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(ContainElement(
			HaveNamespace(Equal(namespace)),
		))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func FBLogsFromNamespaceNotDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(HaveFlatFluentBitLogs(Not(ContainElement(
			HaveNamespace(Equal(namespace)),
		)))))
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func OtelLogsFromNamespaceDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatOtelLogs(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace)))),
		))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func OtelLogsFromNamespaceNotDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			HaveFlatOtelLogs(Not(ContainElement(HaveResourceAttributes(HaveKeyWithValue("k8s.namespace.name", namespace))))),
		))
	}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineHealthy and MetricPipelineHealthy have similarities, but they are not the same
func LogPipelineHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())

		statusConditionHealthy(g, pipeline, conditions.TypeAgentHealthy)
		statusConditionHealthy(g, pipeline, conditions.TypeConfigurationGenerated)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineOtelHealthy and LogPipelineHealthy have similarities, but they are not the same
func LogPipelineOtelHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())

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
func LogPipelineHasCondition(ctx context.Context, k8sClient client.Client, pipelineName string, expectedCond metav1.Condition) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineConditionReasonsTransition,TracePipelineConditionReasonsTransition, MetricPipelineConditionReasonsTransition have similarities, but they are not the same
func LogPipelineConditionReasonsTransition(ctx context.Context, k8sClient client.Client, pipelineName, condType string, expected []ReasonStatus) {
	var currCond *metav1.Condition

	for _, expected := range expected {
		// Wait for the current condition to match the expected condition
		Eventually(func(g Gomega) ReasonStatus {
			var pipeline telemetryv1alpha1.LogPipeline
			key := types.NamespacedName{Name: pipelineName}
			err := k8sClient.Get(ctx, key, &pipeline)
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

func LogPipelineUnsupportedMode(ctx context.Context, k8sClient client.Client, pipelineName string, isUnsupportedMode bool) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(*pipeline.Status.UnsupportedMode).To(Equal(isUnsupportedMode))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
