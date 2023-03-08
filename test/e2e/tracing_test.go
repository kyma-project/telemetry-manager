//go:build e2e

package e2e

import (
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Tracing", func() {
	Context("When no TracePipeline exists", Ordered, func() {
		tracePipeline := &telemetryv1alpha1.TracePipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: telemetryv1alpha1.TracePipelineSpec{
				Output: telemetryv1alpha1.TracePipelineOutput{
					Otlp: &telemetryv1alpha1.OtlpOutput{
						Endpoint: telemetryv1alpha1.ValueType{Value: "http://localhost"},
					},
				},
			},
		}

		It("Should successfully create a TracePipeline", func() {
			Expect(k8sClient.Create(ctx, tracePipeline)).Should(Succeed())
		})

		It("Should have a trace collector Deployment", func() {
			Eventually(func() error {
				var deployment appsv1.Deployment
				deploymentKey := types.NamespacedName{
					Name:      "telemetry-trace-collector",
					Namespace: systemNamespace,
				}
				return k8sClient.Get(ctx, deploymentKey, &deployment)
			}, timeout, interval).Should(Succeed())
		})

		It("Should successfully delete the TracePipeline", func() {
			Expect(k8sClient.Delete(ctx, tracePipeline)).Should(Succeed())
		})
	})
})
