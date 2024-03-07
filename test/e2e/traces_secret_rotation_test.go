//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces Secret Rotation", Label("traces"), func() {
	Context("When tracepipeline with missing secret reference exists", Ordered, func() {
		hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname", kitkyma.DefaultNamespaceName, kitk8s.WithStringData("trace-host", "http://localhost:4317"))
		tracePipeline := kitk8s.NewTracePipeline("without-secret").WithOutputEndpointFromSecret(hostSecret.SecretKeyRef("trace-host"))

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipeline.K8sObject())).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, tracePipeline.K8sObject(), hostSecret.K8sObject())).Should(Succeed())
			})
		})

		It("Should set ConfigurationGenerated condition to false and Pending condition to true", func() {
			Eventually(func(g Gomega) {
				var fetched telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: tracePipeline.Name()}
				g.Expect(k8sClient.Get(ctx, key, &fetched)).To(Succeed())

				configurationGeneratedCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeConfigurationGenerated)
				g.Expect(configurationGeneratedCond).NotTo(BeNil())
				g.Expect(configurationGeneratedCond.Status).Should(Equal(metav1.ConditionFalse))
				g.Expect(configurationGeneratedCond.Reason).Should(Equal(conditions.ReasonReferencedSecretMissing))

				pendingCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypePending)
				g.Expect(pendingCond).NotTo(BeNil())
				g.Expect(pendingCond.Status).Should(Equal(metav1.ConditionTrue))
				g.Expect(pendingCond.Reason).Should(Equal(conditions.ReasonReferencedSecretMissing))

				runningCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeRunning)
				g.Expect(runningCond).To(BeNil())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not have trace gateway deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.TraceGatewayName, &deployment)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue())
		})

		It("Should have running tracepipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, hostSecret.K8sObject())).Should(Succeed())
			})

			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, tracePipeline.Name())
		})
	})

})
