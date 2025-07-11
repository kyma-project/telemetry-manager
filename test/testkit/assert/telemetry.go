package assert

import (
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TelemetryHasState(t testkit.T, expectedState operatorv1alpha1.State) {
	t.Helper()

	Eventually(func(g Gomega) {
		var telemetryCR operatorv1alpha1.Telemetry
		g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetryCR)).To(Succeed())
		g.Expect(telemetryCR.Status.State).To(Equal(expectedState))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TelemetryHasCondition(t testkit.T, k8sClient client.Client, expectedCond metav1.Condition) {
	t.Helper()

	Eventually(func(g Gomega) {
		var telemetryCR operatorv1alpha1.Telemetry

		res := types.NamespacedName{Name: "default", Namespace: kitkyma.SystemNamespaceName}
		g.Expect(k8sClient.Get(t.Context(), res, &telemetryCR)).To(Succeed())
		condition := meta.FindStatusCondition(telemetryCR.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func SelfMonitorIsHealthyForPipeline(t testkit.T, k8sClient client.Client, pipelineName string) {
	t.Helper()

	Eventually(func(g Gomega) {
		var pipeline telemetryv1alpha1.LogPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(t.Context(), key, &pipeline)).To(Succeed())
		g.Expect(meta.IsStatusConditionTrue(pipeline.Status.Conditions, conditions.TypeFlowHealthy)).To(BeTrueBecause("Flow not healthy"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
