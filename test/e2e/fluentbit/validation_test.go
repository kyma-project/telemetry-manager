//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	Context("When a validating webhook exists", Ordered, func() {
		BeforeAll(func() {
			Eventually(func(g Gomega) {
				var validatingWebhookConfiguration admissionregistrationv1.ValidatingWebhookConfiguration
				g.Expect(K8sClient.Get(Ctx, client.ObjectKey{Name: kitkyma.ValidatingWebhookName}, &validatingWebhookConfiguration)).Should(Succeed())
				g.Expect(validatingWebhookConfiguration.Webhooks).Should(HaveLen(2))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reject a logpipeline with denied custom filter", func() {
			logPipeline := testutils.NewLogPipelineBuilder().
				WithName("denied-custom-filter-pipeline").
				WithCustomFilter("Name kubernetes").
				WithCustomOutput("Name stdout").
				Build()
			Consistently(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(Ctx, K8sClient, &logPipeline)).ShouldNot(Succeed())
			}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should reject a logpipeline with misconfigured secretrefs", func() {
			logPipeline := testutils.NewLogPipelineBuilder().
				WithName("misconfigured-secretref-pipeline").
				WithHTTPOutput(testutils.HTTPBasicAuthFromSecret("name", "namespace", "", "")).
				Build()
			Consistently(func(g Gomega) {
				g.Expect(kitk8s.CreateObjects(Ctx, K8sClient, &logPipeline)).ShouldNot(Succeed())
			}, periodic.ConsistentlyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
