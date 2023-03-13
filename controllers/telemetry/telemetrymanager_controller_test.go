package telemetry

import (
	"context"
	"fmt"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deploying a TelemetryManager", func() {
	const (
		timeout  = time.Second * 100
		interval = time.Millisecond * 250
	)

	When("creating TelemetryManager", func() {
		ctx := context.Background()

		telemetryManager := &telemetryv1alpha1.TelemetryManager{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummy",
			},
		}

		It("creates TelemetryManager resource", func() {
			Expect(k8sClient.Create(ctx, telemetryManager)).Should(Succeed())

			Eventually(func() error {
				telemetryManagerLookupKey := types.NamespacedName{
					Name: "dummy",
				}
				var telemetryManager telemetryv1alpha1.TelemetryManager
				err := k8sClient.Get(ctx, telemetryManagerLookupKey, &telemetryManager)
				if err != nil {
					return err
				}

				if err := validateStatus(telemetryManager.Status); err != nil {
					return err
				}

				return nil
			}, timeout, interval).Should(BeNil())

			Expect(k8sClient.Delete(ctx, telemetryManager)).Should(Succeed())
		})
	})
})

func validateStatus(status telemetryv1alpha1.TelemetryManagerStatus) error {
	if status.State != telemetryv1alpha1.StateReady {
		return fmt.Errorf("unexpected state: %s", status.State)
	}
	return nil
}
