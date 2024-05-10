package assert

import (
	"context"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func MetricsFromNamespaceDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string, metricNames []string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			ContainMd(SatisfyAll(
				ContainMetric(WithName(BeElementOf(metricNames))),
				ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", namespace)),
			)),
		))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func MetricsFromNamespaceNotDelivered(proxyClient *apiserverproxy.Client, backendExportURL, namespace string) {
	Consistently(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(
			Not(ContainMd(
				ContainResourceAttrs(HaveKeyWithValue("k8s.namespace.name", namespace)),
			)),
		))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, periodic.TelemetryConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

func MetricPipelineHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeGatewayHealthy)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeAgentHealthy)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func MetricPipelineNotHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeGatewayHealthy)).To(BeTrue())
		g.Expect(meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func MetricPipelineHasCondition(ctx context.Context, k8sClient client.Client, pipelineName string, expectedCond metav1.Condition) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, expectedCond.Type)
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
