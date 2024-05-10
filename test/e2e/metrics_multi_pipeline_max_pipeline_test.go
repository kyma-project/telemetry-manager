//go:build e2e

package e2e

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfMetricPipelines = 3

		var (
			pipelinesNames       = make([]string, 0, maxNumberOfMetricPipelines)
			pipelineCreatedFirst *telemetryv1alpha1.MetricPipeline
			pipelineCreatedLater *telemetryv1alpha1.MetricPipeline
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := 0; i < maxNumberOfMetricPipelines; i++ {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				pipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName)
				pipelinesNames = append(pipelinesNames, pipelineName)

				objs = append(objs, pipeline.K8sObject())

				if i == 0 {
					pipelineCreatedFirst = pipeline.K8sObject()
				}
			}

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				k8sObjectsToDelete := slices.DeleteFunc(k8sObjects, func(obj client.Object) bool {
					return obj.GetName() == pipelineCreatedFirst.GetName() //first pipeline is deleted separately in one of the specs
				})
				k8sObjectsToDelete = append(k8sObjectsToDelete, pipelineCreatedLater)
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjectsToDelete...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipelineName := range pipelinesNames {
				assert.MetricPipelineHealthy(ctx, k8sClient, pipelineName)
			}
		})

		It("Should set ConfigurationGenerated condition to false", func() {
			By("Creating an additional pipeline", func() {
				pipelineName := fmt.Sprintf("%s-limit-exceeding", suite.ID())
				pipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName)
				pipelineCreatedLater = pipeline.K8sObject()
				pipelinesNames = append(pipelinesNames, pipelineName)

				assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineName, metav1.Condition{
					Type:   conditions.TypeConfigurationGenerated,
					Status: metav1.ConditionFalse,
					Reason: conditions.ReasonMaxPipelinesExceeded,
				})
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineCreatedFirst)).Should(Succeed())

				for _, pipeline := range pipelinesNames[1:] {
					assert.MetricPipelineHealthy(ctx, k8sClient, pipeline)
				}
			})
		})
	})

})
