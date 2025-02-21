//go:build e2e

package fluentbit

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelMaxPipeline), Ordered, func() {

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfLogPipelines = 5

		var (
			pipelinesNames = make([]string, 0, maxNumberOfLogPipelines)
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := range maxNumberOfLogPipelines {
				pipelineName := fmt.Sprintf("%s-limit-%d", ID(), i)
				pipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).Build()
				pipelinesNames = append(pipelinesNames, pipelineName)

				objs = append(objs, &pipeline)
			}

			return objs
		}

		BeforeAll(func() {
			K8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipelineName := range pipelinesNames {
				assert.LogPipelineHealthy(Ctx, K8sClient, pipelineName)
			}
		})

		It("Should reject logpipeline creation after reaching max logpipeline", func() {
			By("Creating an additional pipeline", func() {
				pipelineName := fmt.Sprintf("%s-limit-exceeding", ID())
				pipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).Build()

				Expect(kitk8s.CreateObjects(Ctx, K8sClient, &pipeline)).ShouldNot(Succeed())
			})
		})
	})
})
