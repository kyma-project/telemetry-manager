package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"

)

var _ = Describe("Traces Secret Rotation", Label("tracing"), func() {
	Context("When tracepipeline with missing secret reference exists", Ordered, func() {
		hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname", kitkyma.DefaultNamespaceName, kitk8s.WithStringData("trace-host", "http://localhost:4317"))
		tracePipeline := kittrace.NewPipeline("without-secret").WithOutputEndpointFromSecret(hostSecret.SecretKeyRef("trace-host"))

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipeline.K8sObject())).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, tracePipeline.K8sObject(), hostSecret.K8sObject())).Should(Succeed())
			})
		})

		It("Should have pending tracepipeline", func() {
			verifiers.TracePipelineShouldStayPending(ctx, k8sClient, tracePipeline.Name())
		})

		It("Should have running tracepipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, hostSecret.K8sObject())).Should(Succeed())
			})

			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, tracePipeline.Name())
		})
	})

})
