//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
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
			hostKey := "log-host"
			httpHostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData(hostKey, "http://log-host:9880")).K8sObject()
			objs = append(objs, httpHostSecret)
			for i := 0; i < maxNumberOfLogPipelines; i++ {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				pipeline := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithHTTPOutput(testutils.HTTPHostFromSecret(httpHostSecret.Name, httpHostSecret.Namespace, hostKey)).
					Build()
				pipelinesNames = append(pipelinesNames, pipelineName)

				objs = append(objs, &pipeline)
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
				hostKey := "log-host"
				pipelineHostSecret := kitk8s.NewOpaqueSecret("http-hostname", kitkyma.DefaultNamespaceName,
					kitk8s.WithStringData(hostKey, "http://log-host:9880")).K8sObject()

				pipeline := testutils.NewLogPipelineBuilder().
					WithName(pipelineName).
					WithHTTPOutput(testutils.HTTPHostFromSecret(pipelineHostSecret.Name, pipelineHostSecret.Namespace, hostKey)).
					Build()

				Expect(kitk8s.CreateObjects(ctx, k8sClient, &pipeline, pipelineHostSecret)).ShouldNot(Succeed())
			})
		})
	})
})
