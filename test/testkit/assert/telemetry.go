package assert

import (
	"context"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

func TelemetryHasCondition(ctx context.Context, k8sClient client.Client, conditionType, tlsReason string, status bool) {
	Eventually(func(g Gomega) {
		var telemetryCR operatorv1alpha1.Telemetry
		res := types.NamespacedName{Name: "default", Namespace: kitkyma.SystemNamespaceName}
		g.Expect(k8sClient.Get(ctx, res, &telemetryCR)).To(Succeed())
		g.Expect(telemetryCR.Status.State).To(Equal(operatorv1alpha1.StateWarning))
		g.Expect(meta.IsStatusConditionTrue(telemetryCR.Status.Conditions, conditionType)).To(Equal(status))
		condition := meta.FindStatusCondition(telemetryCR.Status.Conditions, conditionType)
		g.Expect(condition.Reason).To(Equal(tlsReason))

	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}
