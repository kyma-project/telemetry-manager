package operator

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/reconciler"

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
				Name:      "telemetry",
				Namespace: "default",
			},
		}

		It("telemetry resource status should be ready", func() {
			Expect(k8sClient.Create(ctx, telemetryTestObj)).Should(Succeed())

			Eventually(func() error {
				telemetryLookupKey := types.NamespacedName{
					Name:      "telemetry",
					Namespace: "default",
				}
				var telemetryTestInstance operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, telemetryLookupKey, &telemetryTestInstance)
				if err != nil {
					return err
				}

				return validateStatus(telemetryTestInstance.Status)
			}, timeout, interval).Should(BeNil())
			Expect(k8sClient.Delete(ctx, telemetryTestObj)).Should(Succeed())
		})

		It("has all sub-resource status", func() {
			Eventually(func() bool {
				telemetryLookupKey := types.NamespacedName{
					Name:      "telemetry",
					Namespace: "default",
				}
				var telemetryTestInstance operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, telemetryLookupKey, &telemetryTestInstance)
				Expect(err).ToNot(HaveOccurred())
				conditions := telemetryTestInstance.Status.Conditions
				Expect(validateLoggingCondition(&conditions, reconciler.ReasonNoPipelineDeployed)).To(BeTrue())
				Expect(validateTracingCondition(&conditions, reconciler.ReasonNoPipelineDeployed)).To(BeTrue())
				Expect(validateMetricCondition(&conditions, reconciler.ReasonNoPipelineDeployed)).To(BeTrue())
				return true
			}, timeout, interval).Should(BeTrue())
		})

	})
})

func validateStatus(status operatorv1alpha1.TelemetryStatus) error {
	if status.State != operatorv1alpha1.StateReady {
		return fmt.Errorf("unexpected state: %s", status.State)
	}
	return nil
}

func validateLoggingCondition(conditions *[]metav1.Condition, status metav1.ConditionStatus) bool {
	for _, c := range *conditions {
		if c.Type == reconciler.LogConditionType && c.Status == status {
			return true
		}
	}
	return false
}
func validateTracingCondition(conditions *[]metav1.Condition, status metav1.ConditionStatus) bool {
	for _, c := range *conditions {
		if c.Type == reconciler.TraceConditionType && c.Status == status {
			return true
		}
	}
	return false
}

func validateMetricCondition(conditions *[]metav1.Condition, status metav1.ConditionStatus) bool {
	for _, c := range *conditions {
		if c.Type == reconciler.MetricConditionType && c.Status == status {
			return true
		}
	}
	return false
}

//func validateEndpoint(endpoint operatorv1alpha1.Endpoints, expected)
