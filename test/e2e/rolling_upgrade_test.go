package e2e

import (
	"fmt"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe(suite.ID(), Label(suite.LabelIntegration), func() {

	Context("When performing a rolling upgrade", func() {
		makeResources := func() []client.Object {
			var objs []client.Object
			logPipelineName := fmt.Sprintf("%s-log", suite.ID())
			metricPipelineName := fmt.Sprintf("%s-metric", suite.ID())
			tracePipelineName := fmt.Sprintf("%s-trace", suite.ID())

			logPipeline := testutils.NewLogPipelineBuilder().WithName(logPipelineName).Build()
			metricPipeline := testutils.NewMetricPipelineBuilder().WithName(metricPipelineName).Build()
			tracePipeline := testutils.NewTracePipelineBuilder().WithName(tracePipelineName).Build()

			objs = append(objs, &logPipeline, &metricPipeline, &tracePipeline)

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should be upgraded", func() {

		})
	})

})
