package assert

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TelemetryCRExists(t *testing.T) {
	t.Helper()

	Eventually(func(g Gomega) {
		var telemetryCR operatorv1beta1.Telemetry
		g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetryCR)).To(Succeed())
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TelemetryHasState(t *testing.T, expectedState operatorv1beta1.State) {
	t.Helper()

	Eventually(func(g Gomega) {
		var telemetryCR operatorv1beta1.Telemetry
		g.Expect(suite.K8sClient.Get(t.Context(), kitkyma.TelemetryName, &telemetryCR)).To(Succeed())
		g.Expect(telemetryCR.Status.State).To(Equal(expectedState))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func TelemetryHasCondition(t *testing.T, k8sClient client.Client, expectedCond metav1.Condition) {
	t.Helper()

	Eventually(func(g Gomega) {
		var telemetryCR operatorv1beta1.Telemetry

		res := types.NamespacedName{Name: "default", Namespace: kitkyma.SystemNamespaceName}
		g.Expect(k8sClient.Get(t.Context(), res, &telemetryCR)).To(Succeed())
		condition := meta.FindStatusCondition(telemetryCR.Status.Conditions, expectedCond.Type)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(expectedCond.Reason))
		g.Expect(condition.Status).To(Equal(expectedCond.Status))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
