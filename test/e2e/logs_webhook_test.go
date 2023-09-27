//go:build e2e

package e2e

import (
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

var _ = Describe("Logs Validating Webhook", Label("logging"), func() {
	Context("When a validating webhook exists", Ordered, func() {
		BeforeAll(func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: webhookName}, &validatingWebhookConfiguration)).Should(Succeed())
				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))
			}, periodic.DefaultTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reject a logpipeline with unknown custom filter", func() {
			logPipeline := kitlog.NewPipeline("unknown-custom-filter-pipeline").WithStdout().WithFilter("Name unknown")
			Expect(kitk8s.CreateObjects(ctx, k8sClient, logPipeline.K8sObject())).ShouldNot(Succeed())
		})

		It("Should reject a logpipeline with denied custom filter", func() {
			logPipeline := kitlog.NewPipeline("denied-custom-filter-pipeline").WithStdout().WithFilter("Name kubernetes")
			Expect(kitk8s.CreateObjects(ctx, k8sClient, logPipeline.K8sObject())).ShouldNot(Succeed())
		})

	})
})
