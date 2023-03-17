package operator

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deploying a Telemetry", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	When("creating Telemetry", func() {
		ctx := context.Background()

		telemetryTestObj := &operatorv1alpha1.Telemetry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "telemetry",
			},
		}

		It("telemetry resource status should be ready", func() {
			Expect(k8sClient.Create(ctx, telemetryTestObj)).Should(Succeed())

			Eventually(func() error {
				telemetryLookupKey := types.NamespacedName{
					Name: "telemetry",
				}
				var telemetryTestInstance operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, telemetryLookupKey, &telemetryTestInstance)
				if err != nil {
					return err
				}

				if err := validateStatus(telemetryTestInstance.Status); err != nil {
					return err
				}

				return nil
			}, timeout, interval).Should(BeNil())
			Expect(k8sClient.Delete(ctx, telemetryTestObj)).Should(Succeed())
		})
	})
})

func validateStatus(status operatorv1alpha1.TelemetryStatus) error {
	if status.State != operatorv1alpha1.StateReady {
		return fmt.Errorf("unexpected state: %s", status.State)
	}
	return nil
}
