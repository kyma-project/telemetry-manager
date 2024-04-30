package verifiers

import (
	"context"
	"net/http"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/apiserverproxy"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func LogsShouldBeDelivered(proxyClient *apiserverproxy.Client, expectedPodNamePrefix string, backendExportURL string) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(backendExportURL)
		g.Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainLd(ContainLogRecord(
			WithPodName(ContainSubstring(expectedPodNamePrefix))),
		)))
	}, periodic.EventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}

//nolint:dupl //LogPipelineShouldBeHealthy and MetricPipelineShouldBeHealthy have similarities, but they are not the same
func LogPipelineShouldBeHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeAgentHealthy)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeRunning)).To(BeTrue())
		g.Expect(meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypePending)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl // This provides a better readability for the test as we know pipeline should not be healthy
func LogPipelineShouldNotBeHealthy(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeAgentHealthy)).To(BeTrue())
		g.Expect(meta.IsStatusConditionFalse(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)).To(BeTrue())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypePending)).To(BeTrue())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

//nolint:dupl // This provides a better readability for the test as we can test the TLS condition in a clear way
func LogPipelineShouldHaveTLSCondition(ctx context.Context, k8sClient client.Client, pipelineName string, tlsCondition string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		condition := meta.FindStatusCondition(pipeline.Status.Conditions, conditions.TypeConfigurationGenerated)
		g.Expect(condition.Reason).To(Equal(tlsCondition))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func LogPipelineShouldHaveLegacyConditionsAtEnd(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())

		conditionsSize := len(pipeline.Status.Conditions)

		pendingCond := pipeline.Status.Conditions[conditionsSize-2]
		g.Expect(pendingCond.Type).To(Equal(conditions.TypePending))

		runningCond := pipeline.Status.Conditions[conditionsSize-1]
		g.Expect(runningCond.Type).To(Equal(conditions.TypeRunning))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
