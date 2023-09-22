package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics", Label("metrics"), func() {
	Context("When a metricpipeline with missing secret reference exists", Ordered, func() {
		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", kitkyma.DefaultNamespaceName,
			kitk8s.WithStringData("metric-host", "http://localhost:4317"))
		metricPipeline := kitmetric.NewPipeline("without-secret", hostSecret.SecretKeyRef("metric-host"))

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipeline.K8sObject())).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, metricPipeline.K8sObject(), hostSecret.K8sObject())).Should(Succeed())
			})
		})

		It("Should have pending metricpipeline", func() {
			verifiers.MetricPipelineShouldStayPending(ctx, k8sClient, metricPipeline.Name())
		})

		It("Should not have metric-gateway deployment", func() {
			Consistently(func(g Gomega) {
				var deployment appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)).To(Succeed())
			}, reconciliationTimeout, interval).ShouldNot(Succeed())
		})

		It("Should have running metricpipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, hostSecret.K8sObject())).Should(Succeed())
			})

			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, metricPipeline.Name())
		})
	})
})
