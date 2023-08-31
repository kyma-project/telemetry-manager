//go:build e2e

package e2e

import (
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	webhookName                = "validation.webhook.telemetry.kyma-project.io"
	telemetryTestK8SObjectName = "telemetry-test"
	webhookCertSecret          = types.NamespacedName{
		Name:      "telemetry-webhook-cert",
		Namespace: kymaSystemNamespaceName,
	}
)

var _ = Describe("Telemetry-module", Label("logging", "tracing", "metrics"), Ordered, func() {
	Context("After creating Telemetry resources", Ordered, func() {
		It("Should have ValidatingWebhookConfiguration", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())

				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))

				logPipelineWebhook := validatingWebhookConfiguration.Webhooks[0]
				g.Expect(logPipelineWebhook.Name).Should(Equal("validation.logpipelines.telemetry.kyma-project.io"))
				g.Expect(logPipelineWebhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
				g.Expect(logPipelineWebhook.ClientConfig.Service.Name).Should(Equal("telemetry-operator-webhook"))
				g.Expect(logPipelineWebhook.ClientConfig.Service.Namespace).Should(Equal(kymaSystemNamespaceName))
				g.Expect(*logPipelineWebhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
				g.Expect(*logPipelineWebhook.ClientConfig.Service.Path).Should(Equal("/validate-logpipeline"))
				g.Expect(logPipelineWebhook.Rules).Should(HaveLen(1))
				g.Expect(logPipelineWebhook.Rules[0].Resources).Should(ContainElement("logpipelines"))
				g.Expect(logPipelineWebhook.Rules[0].Operations).Should(ContainElement(admissionv1.Create))
				g.Expect(logPipelineWebhook.Rules[0].Operations).Should(ContainElement(admissionv1.Update))

				logParserWebhook := validatingWebhookConfiguration.Webhooks[1]
				g.Expect(logParserWebhook.Name).Should(Equal("validation.logparsers.telemetry.kyma-project.io"))
				g.Expect(logParserWebhook.ClientConfig.CABundle).ShouldNot(BeEmpty())
				g.Expect(logParserWebhook.ClientConfig.Service.Name).Should(Equal("telemetry-operator-webhook"))
				g.Expect(logParserWebhook.ClientConfig.Service.Namespace).Should(Equal(kymaSystemNamespaceName))
				g.Expect(*logParserWebhook.ClientConfig.Service.Port).Should(Equal(int32(443)))
				g.Expect(*logParserWebhook.ClientConfig.Service.Path).Should(Equal("/validate-logparser"))
				g.Expect(logParserWebhook.Rules).Should(HaveLen(1))
				g.Expect(logParserWebhook.Rules[0].Resources).Should(ContainElement("logparsers"))
				g.Expect(logParserWebhook.Rules[0].Operations).Should(ContainElement(admissionv1.Create))
				g.Expect(logParserWebhook.Rules[0].Operations).Should(ContainElement(admissionv1.Update))
			}, timeout, interval).Should(Succeed())
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
			}, timeout, interval).Should(Succeed())
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
			}, timeout, interval).Should(Succeed())
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
			}, timeout, interval).Should(Succeed())
		})

		AfterAll(func() {
			// Re-create Telemetry to have ValidatingWebhookConfiguration for remaining tests
			Eventually(func(g Gomega) {
				newTelemetry := []client.Object{kitk8s.NewTelemetry("default", "kyma-system").K8sObject()}
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, newTelemetry...)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				var telemetry v1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(v1alpha1.StateReady))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have Telemetry resource", func() {
			Eventually(func(g Gomega) {
				var telemetry v1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(v1alpha1.StateReady))
			}, timeout, interval).Should(Succeed())
		})

		It("Should not delete Telemetry when LogPipeline exists", func() {
			By("Deleting telemetry", func() {
				Expect(kitk8s.ForceDeleteObjects(ctx, k8sClient, telemetryK8sObjects...)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry v1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).Should(Succeed())
				g.Expect(telemetry.Status.State).Should(Equal(v1alpha1.StateError))
				g.Expect(telemetry.Finalizers).Should(HaveLen(1))
				g.Expect(telemetry.Finalizers[0]).Should(Equal("telemetry.kyma-project.io/finalizer"))
			}, timeout, interval).Should(Succeed())
		})

		It("Should reconcile ValidatingWebhookConfiguration if LogPipeline exists", func() {
			testWebhookReconciliation()
		})

		It("Should delete Telemetry", func() {
			By("Deleting Telemetry and other resources", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sLogPipelineObject...)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var telemetry v1alpha1.Telemetry
				g.Expect(k8sClient.Get(ctx, telemetryKey, &telemetry)).ShouldNot(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		It("Should not have Webhook and CA bundle", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
			}, timeout, interval).ShouldNot(Succeed())

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(Succeed())
			}, timeout, interval).ShouldNot(Succeed())
		})
	})
})

func testWebhookReconciliation() {
	var oldUID types.UID
	By("Deleting ValidatingWebhookConfiguration", func() {
		var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
		oldUID = validatingWebhookConfiguration.UID
		Expect(k8sClient.Delete(ctx, &validatingWebhookConfiguration)).Should(Succeed())
	})

	var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
		g.Expect(validatingWebhookConfiguration.UID).ShouldNot(Equal(oldUID))
	}, timeout, interval).Should(Succeed())
}

func makeTestPipelineK8sObjects() []client.Object {
	logPipeline := kitlog.NewPipeline(telemetryTestK8SObjectName)
	return []client.Object{
		logPipeline.K8sObject(),
	}
}
