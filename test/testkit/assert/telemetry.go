package assert

import (
	"context"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func TelemetryHasState(ctx context.Context, k8sClient client.Client, expectedState operatorv1alpha1.State) {
	Eventually(func(g Gomega) {
		var telemetryCR operatorv1alpha1.Telemetry
		g.Expect(k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetryCR)).To(Succeed())
		g.Expect(telemetryCR.Status.State).To(Equal(expectedState))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TelemetryHasCondition(ctx context.Context, k8sClient client.Client, expectedCond metav1.Condition) {
	Eventually(func(g Gomega) {
		var telemetryCR operatorv1alpha1.Telemetry

		res := types.NamespacedName{Name: "default", Namespace: kitkyma.SystemNamespaceName}
		g.Expect(k8sClient.Get(ctx, res, &telemetryCR)).To(Succeed())
		condition := meta.FindStatusCondition(telemetryCR.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func SelfMonitorIsHealthyForPipeline(ctx context.Context, k8sClient client.Client, pipelineName string) {
	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
