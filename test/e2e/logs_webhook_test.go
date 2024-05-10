//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			verifiers.WebhookShouldBeHealthy(ctx, k8sClient)
		})
	})

	Context("When a validating webhook exists", Ordered, func() {
		logPipelineUnknownFilter := kitk8s.NewLogPipelineV1Alpha1("unknown-custom-filter-pipeline").WithStdout().WithFilter("Name unknown").K8sObject()
		BeforeAll(func() {

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, logPipelineUnknownFilter)).Should(Succeed())
			})

			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: kitkyma.WebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should accept a logpipeline with unknown custom filter", func() {

			Eventually(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, logPipelineUnknownFilter)).Should(Succeed())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reject a logpipeline with denied custom filter", func() {
			logPipeline := kitk8s.NewLogPipelineV1Alpha1("denied-custom-filter-pipeline").WithStdout().WithFilter("Name kubernetes")
			Consistently(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(ctx, k8sClient, logPipeline.K8sObject())).ShouldNot(Succeed())
			}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

	})
})
