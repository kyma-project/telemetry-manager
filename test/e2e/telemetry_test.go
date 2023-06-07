//go:build e2e

package e2e

import (
	"github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	v1alpha12 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/log"
	kittelemetry "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/telemetry"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	webhookName                = "validation.webhook.telemetry.kyma-project.io"
	telemetryTestK8SObjectName = "telemetry-test"
	webhookCertSecret          = types.NamespacedName{
		Name:      "telemetry-webhook-cert",
		Namespace: kymaSystemNamespaceName,
	}

	resourceKey = types.NamespacedName{
		Name:      telemetryTestK8SObjectName,
		Namespace: kymaSystemNamespaceName,
	}
)

var _ = Describe("Telemetry-module", func() {

	Context("After creating telemetry resources", Ordered, func() {

		BeforeAll(func() {
			k8sObjects := makeTelemetryTestK8sObjects()
			k8sLogPipelineObject := makeTestPipelineK8sObjects()
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sLogPipelineObject...)).Should(Succeed())
		})

		It("Should have ValidatingWebhookkConfiguration", func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(BeNil())

				g.Expect(validatingWebhookConfiguration.OwnerReferences).Should(HaveLen(1))
				g.Expect(validatingWebhookConfiguration.OwnerReferences[0].Name).Should(Equal("default"))
				g.Expect(validatingWebhookConfiguration.OwnerReferences[0].Kind).Should(Equal("Telemetry"))

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
				g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(BeNil())
				g.Expect(secret.OwnerReferences).Should(HaveLen(1))
				g.Expect(secret.OwnerReferences[0].Name).Should(Equal("default"))
				g.Expect(secret.OwnerReferences[0].Kind).Should(Equal("Telemetry"))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.crt", Not(BeEmpty())))
				g.Expect(secret.Data).Should(HaveKeyWithValue("ca.key", Not(BeEmpty())))
			}, timeout, interval).Should(Succeed())
		})

		It("Should reconcile ValidatingWebhookConfiguration", func() {
			var oldUid types.UID
			By("Deleting ValidatingWebhookConfiguration", func() {
				var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
				Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(BeNil())
				oldUid = validatingWebhookConfiguration.UID
				Expect(k8sClient.Delete(ctx, &validatingWebhookConfiguration)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(BeNil())
					g.Expect(validatingWebhookConfiguration.OwnerReferences).Should(HaveLen(1))
					g.Expect(validatingWebhookConfiguration.UID).ShouldNot(Equal(oldUid))
				}, timeout, interval).Should(Succeed())
			})
		})

		It("Should reconcile CA bundle secret", func() {
			var oldUid types.UID
			By("Deleting secret", func() {
				var secret corev1.Secret
				Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(BeNil())
				oldUid = secret.UID
				Expect(k8sClient.Delete(ctx, &secret)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				Eventually(func(g Gomega) {
					var secret corev1.Secret
					g.Expect(k8sClient.Get(ctx, webhookCertSecret, &secret)).Should(BeNil())
					g.Expect(secret.OwnerReferences).Should(HaveLen(1))
					g.Expect(secret.UID).ShouldNot(Equal(oldUid))
				}, timeout, interval).Should(Succeed())
			})
		})

		It("Should not delete telemetry since LogPipeline in use", func() {
			By("Deleting telemetry", func() {
				var telemetry v1alpha1.Telemetry
				Expect(k8sClient.Get(ctx, resourceKey, &telemetry)).Should(BeNil())
				Expect(k8sClient.Delete(ctx, &telemetry)).Should(BeNil())
			})

			Eventually(func(g Gomega) {
				Eventually(func(g Gomega) {
					var telemetry v1alpha1.Telemetry
					g.Expect(k8sClient.Get(ctx, resourceKey, &telemetry)).Should(Succeed())
					g.Expect(telemetry.Status).Should(Equal(v1alpha1.StateError))
					g.Expect(telemetry.OwnerReferences).Should(HaveLen(1))
				}, timeout, interval).Should(Succeed())
			})
		})

		It("Should delete telemetry since no orphan resource remain", func() {
			By("Deleting telemetry and other resources", func() {
				var logPipeline v1alpha12.LogPipeline
				Expect(k8sClient.Get(ctx, resourceKey, &logPipeline)).Should(BeNil())
				Expect(k8sClient.Delete(ctx, &logPipeline)).Should(BeNil())
			})

			Eventually(func(g Gomega) {
				Eventually(func(g Gomega) {
					var telemetry v1alpha1.Telemetry
					g.Expect(k8sClient.Get(ctx, resourceKey, &telemetry)).Should(BeNil())
					g.Expect(telemetry.Status).Should(Equal(v1alpha1.StateDeleting))
					telemetryObject := kittelemetry.NewTelemetry("default")

					k8sObjects := []client.Object{
						telemetryObject.K8sObject(),
					}
					g.Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				}, timeout, interval).Should(Succeed())
			})
		})
	})

})

func makeTelemetryTestK8sObjects() []client.Object {
	telemetry := kittelemetry.NewTelemetry(telemetryTestK8SObjectName)
	return []client.Object{
		telemetry.K8sObject(),
	}
}

func makeTestPipelineK8sObjects() []client.Object {
	logPipeline := kitlog.NewPipeline(telemetryTestK8SObjectName)
	return []client.Object{
		logPipeline.K8sObject(),
	}
}
