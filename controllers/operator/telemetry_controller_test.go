package operator

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"

	"github.com/kyma-project/telemetry-manager/internal/reconciler"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deploying a Telemetry", Ordered, func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	const telemetryNamespace = "default"

	Context("No dependent resources exist", Ordered, func() {
		const telemetryName = "telemetry-1"

		BeforeAll(func() {
			ctx := context.Background()
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
		})

		It("Telemetry status should be ready", func() {
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
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateReady))
		})
	})

	Context("Running TracePipeline exist", Ordered, func() {
		const telemetryName = "telemetry-2"

		BeforeAll(func() {
			ctx := context.Background()
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			runningTracePipeline := testutils.NewTracePipelineBuilder().Build()

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, &runningTracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, &runningTracePipeline)).Should(Succeed())
			runningTracePipeline.Status.SetCondition(telemetryv1alpha1.TracePipelineCondition{
				Reason: reconciler.ReasonTraceGatewayDeploymentReady,
				Type:   telemetryv1alpha1.TracePipelineRunning,
			})
			Expect(k8sClient.Status().Update(ctx, &runningTracePipeline)).Should(Succeed())
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
		})

		It("Telemetry status should be ready", func() {
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
			}, timeout, interval).Should(Equal(operatorv1alpha1.StateReady))
		})
	})

	Context("Pending TracePipeline exist", Ordered, func() {
		const telemetryName = "telemetry-3"

		BeforeAll(func() {
			ctx := context.Background()
			telemetry := &operatorv1alpha1.Telemetry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      telemetryName,
					Namespace: telemetryNamespace,
				},
			}
			pendingTracePipeline := testutils.NewTracePipelineBuilder().Build()

			DeferCleanup(func() {
				Expect(k8sClient.Delete(ctx, &pendingTracePipeline)).Should(Succeed())
				Expect(k8sClient.Delete(ctx, telemetry)).Should(Succeed())
			})
			Expect(k8sClient.Create(ctx, &pendingTracePipeline)).Should(Succeed())
			pendingTracePipeline.Status.SetCondition(telemetryv1alpha1.TracePipelineCondition{
				Reason: reconciler.ReasonTraceGatewayDeploymentNotReady,
				Type:   telemetryv1alpha1.TracePipelinePending,
			})
			Expect(k8sClient.Status().Update(ctx, &pendingTracePipeline)).Should(Succeed())
			Expect(k8sClient.Create(ctx, telemetry)).Should(Succeed())
		})

		It("Telemetry status should be warning", func() {
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
