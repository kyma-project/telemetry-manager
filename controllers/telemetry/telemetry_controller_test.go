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

var _ = Describe("Deploying a Telemetry", func() {
	const (
		timeout  = time.Second * 100
		interval = time.Millisecond * 250
	)

	When("creating Telemetry", func() {
		ctx := context.Background()

		telemetryDummy := &telemetryv1alpha1.Telemetry{
			ObjectMeta: metav1.ObjectMeta{
				Name: "dummy",
			},
		}

		It("creates telemetry resource", func() {
			Expect(k8sClient.Create(ctx, telemetryDummy)).Should(Succeed())

			Eventually(func() error {
				telemetryLookupKey := types.NamespacedName{
					Name: "dummy",
				}
				var telemetryObj telemetryv1alpha1.Telemetry
				err := k8sClient.Get(ctx, telemetryLookupKey, &telemetryObj)
				if err != nil {
					return err
				}

				if err := validateStatus(telemetryObj.Status); err != nil {
					return err
				}

				return nil
			}, timeout, interval).Should(BeNil())

			Expect(k8sClient.Delete(ctx, telemetryDummy)).Should(Succeed())
		})
	})
})

func validateStatus(status telemetryv1alpha1.TelemetryStatus) error {
	if status.State != telemetryv1alpha1.StateReady {
		return fmt.Errorf("unexpected state: %s", status.State)
	}
	return nil
}
