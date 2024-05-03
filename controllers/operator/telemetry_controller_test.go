package operator

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
)

var _ = Describe("Deploying a Telemetry", Ordered, func() {
	const (
		timeout            = time.Second * 10
		interval           = time.Millisecond * 250
		telemetryNamespace = "default"
	)

	Context("When a pending TracePipeline exists", Ordered, func() {
		const telemetryName = "telemetry-3"

		BeforeAll(func() {
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			tracePipeline := testutils.NewTracePipelineBuilder().Build()

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, &tracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
			Expect(k8sClient.Create(ctx, &tracePipeline)).Should(Succeed())

			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeGatewayHealthy, Status: metav1.ConditionFalse, Reason: conditions.ReasonDeploymentNotReady})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypeConfigurationGenerated, Status: metav1.ConditionTrue, Reason: conditions.ReasonConfigurationGenerated})
			meta.SetStatusCondition(&tracePipeline.Status.Conditions, metav1.Condition{Type: conditions.TypePending, Status: metav1.ConditionTrue, Reason: conditions.ReasonTraceGatewayDeploymentNotReady})
			Expect(k8sClient.Status().Update(ctx, &tracePipeline)).Should(Succeed())
		})

		It("Should have Telemetry with warning state", func() {
			Eventually(func() (operatorv1alpha1.State, error) {
				lookupKey := types.NamespacedName{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				}
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, lookupKey, &telemetry)
				if err != nil {
					return "", err
				}
				return telemetry.Status.State, nil
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateWarning))
		})
	})

})
