//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

var (
	webhookName                = "validation.webhook.telemetry.kyma-project.io"
	telemetryTestK8SObjectName = "telemetry-test"
	webhookCertSecret          = types.NamespacedName{
		Name:      "telemetry-webhook-cert",
		Namespace: kitkyma.SystemNamespaceName,
	}
)

var _ = Describe("Telemetry Module", Label("telemetry"), Ordered, func() {
	Context("After creating Telemetry resources", Ordered, func() {
		It("Should have ValidatingWebhookConfiguration", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())

				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))

				logPipelineWebhook := validatingWebhookConfiguration.Webhooks[0]
				g.Expect(logPipelineWebhook.Name).Should(Equal("validation.logpipelines.telemetry.kyma-project.io"))
				g.Expect(logPipelineWebhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
				g.Expect(logPipelineWebhook.ClientConfig.Service.Name).Should(Equal("telemetry-manager-webhook"))
				g.Expect(logPipelineWebhook.ClientConfig.Service.Namespace).Should(Equal(kitkyma.SystemNamespaceName))
				g.Expect(*logPipelineWebhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
				g.Expect(*logPipelineWebhook.ClientConfig.Service.Path).Should(Equal("/validate-logpipeline"))
				g.Expect(logPipelineWebhook.Rules).Should(HaveLen(1))
				g.Expect(logPipelineWebhook.Rules[0].Resources).Should(ContainElement("logpipelines"))
				g.Expect(logPipelineWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Create))
				g.Expect(logPipelineWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Update))

				logParserWebhook := validatingWebhookConfiguration.Webhooks[1]
				g.Expect(logParserWebhook.Name).Should(Equal("validation.logparsers.telemetry.kyma-project.io"))
				g.Expect(logParserWebhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
				g.Expect(logParserWebhook.ClientConfig.Service.Name).Should(Equal("telemetry-manager-webhook"))
				g.Expect(logParserWebhook.ClientConfig.Service.Namespace).Should(Equal(kitkyma.SystemNamespaceName))
				g.Expect(*logParserWebhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
				g.Expect(*logParserWebhook.ClientConfig.Service.Path).Should(Equal("/validate-logparser"))
				g.Expect(logParserWebhook.Rules).Should(HaveLen(1))
				g.Expect(logParserWebhook.Rules[0].Resources).Should(ContainElement("logparsers"))
				g.Expect(logParserWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Create))
				g.Expect(logParserWebhook.Rules[0].Operations).Should(ContainElement(admissionregistrationv1.Update))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have secret with webhook CA bundle", func() {
			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(Succeed())
				g.Expect(secret.OwnerReferences).Should(HaveLen(1))
				g.Expect(secret.OwnerReferences[0].Name).Should(Equal("default"))
				g.Expect(secret.OwnerReferences[0].Kind).Should(Equal("Telemetry"))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.key", Not(BeEmpty())))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reconcile ValidatingWebhookConfiguration", func() {
			testWebhookReconciliation()
		})

		It("Should reconcile CA bundle secret", func() {
			var oldUID types.UID
			By("Deleting secret", func() {
				var secret corev1.Secret
				Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(Succeed())
				oldUID = secret.UID
				Expect(k8sClient.Delete(ctx, &secret)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(Succeed())
				g.Expect(secret.OwnerReferences).Should(HaveLen(1))
				g.Expect(secret.UID).ShouldNot(Equal(oldUID))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})

	Context("Deleting Telemetry resources", Ordered, func() {
		telemetryKey := types.NamespacedName{
			Name:      "default",
			Namespace: "kyma-system",
		}
		k8sLogPipelineObject := makeTestPipelineK8sObjects()

		BeforeAll(func() {
			Eventually(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sLogPipelineObject...)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		AfterAll(func() {
			// Re-create Telemetry to have ValidatingWebhookConfiguration for remaining tests
			Eventually(func(g Gomega) {
				newTelemetry := []client.Object{kitk8s.NewTelemetry("default", "kyma-system").K8sObject()}
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, newTelemetry...)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateReady))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have Telemetry resource", func() {
			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateReady))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reconcile ValidatingWebhookConfiguration if LogPipeline exists", func() {
			testWebhookReconciliation()
		})

		It("Should not delete Telemetry when LogPipeline exists", func() {
			By("Deleting telemetry", func() {
				Expect(kitk8s.ForceDeleteObjects(ctx, k8sClient, telemetryK8sObject)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Finalizers).Should(HaveLen(1))
				g.Expect(telemetry.Finalizers[0]).Should(Equal("telemetry.kyma-project.io/finalizer"))
				g.Expect(telemetry.Status.State).Should(Equal(operatorv1alpha1.StateWarning))
				expectedConditions := map[string]metav1.Condition{
					"LogComponentsHealthy": {
						Status:  "False",
						Reason:  "ResourceBlocksDeletion",
						Message: "The deletion of the module is blocked. To unblock the deletion, delete the following resources: LogPipelines (telemetry-test)",
					},
					"MetricComponentsHealthy": {
						Status:  "True",
						Reason:  "NoPipelineDeployed",
						Message: "No pipelines have been deployed",
					},
					"TraceComponentsHealthy": {
						Status:  "True",
						Reason:  "NoPipelineDeployed",
						Message: "No pipelines have been deployed",
					},
				}
				g.Expect(telemetry.Status.Conditions).Should(HaveLen(3))
				for _, actualCond := range telemetry.Status.Conditions {
					expectedCond := expectedConditions[actualCond.Type]
					g.Expect(expectedCond.Status).Should(Equal(actualCond.Status))
					g.Expect(expectedCond.Reason).Should(Equal(actualCond.Reason))
					g.Expect(expectedCond.Message).Should(Equal(actualCond.Message))
					g.Expect(actualCond.LastTransitionTime).NotTo(BeZero())
				}

			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should delete Telemetry", func() {
			By("Deleting the orphaned LogPipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sLogPipelineObject...)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry operatorv1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).ShouldNot(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should not have Webhook and CA bundle", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(Succeed())
		})
	})
})

func testWebhookReconciliation() {
	var oldUID types.UID
	By("Deleting ValidatingWebhookConfiguration", func() {
		var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
		oldUID = validatingWebhookConfiguration.UID
		Expect(k8sClient.Delete(ctx, &validatingWebhookConfiguration)).Should(Succeed())
	})

	Eventually(func(g Gomega) {
		var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
		g.Expect(validatingWebhookConfiguration.UID).ShouldNot(Equal(oldUID))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

func makeTestPipelineK8sObjects() []client.Object {
	logPipeline := kitk8s.NewLogPipeline(telemetryTestK8SObjectName).WithStdout()
	return []client.Object{
		logPipeline.K8sObject(),
	}
}
