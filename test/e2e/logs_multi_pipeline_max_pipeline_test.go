//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMaxPipeline), Ordered, func() {

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfLogPipelines = 5

		var (
			pipelinesNames = make([]string, 0, maxNumberOfLogPipelines)
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			httpHostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("log-host", "http://log-host:9880"))
			objs = append(objs, httpHostSecret.K8sObject())
			for i := 0; i < maxNumberOfLogPipelines; i++ {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				pipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
					WithSecretKeyRef(httpHostSecret.SecretKeyRefV1Alpha1("log-host")).
					WithHTTPOutput()
				pipelinesNames = append(pipelinesNames, pipelineName)

				objs = append(objs, pipeline.K8sObject())
			}

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipelineName := range pipelinesNames {
				assert.LogPipelineHealthy(ctx, k8sClient, pipelineName)
			}
		})

		It("Should reject logpipeline creation after reaching max logpipeline", func() {
			By("Creating an additional pipeline", func() {
				pipelineName := fmt.Sprintf("%s-limit-exceeding", suite.ID())
				pipelineHostSecret := kitk8s.NewOpaqueSecret("http-hostname", kitkyma.DefaultNamespaceName,
					kitk8s.WithStringData("log-host", "http://log-host:9880"))

				pipeline := kitk8s.NewLogPipelineV1Alpha1(pipelineName).
					WithSecretKeyRef(pipelineHostSecret.SecretKeyRefV1Alpha1("log-host")).
					WithHTTPOutput()

				Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline.K8sObject(), pipelineHostSecret.K8sObject())).ShouldNot(Succeed())
			})
		})
	})

})
