//go:build e2e

package e2e

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), Ordered, func() {

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfTracePipelines = 3
		var (
			pipelinesNames       = make([]string, 0, maxNumberOfTracePipelines)
			pipelineCreatedFirst *telemetryv1alpha1.TracePipeline
			pipelineCreatedLater *telemetryv1alpha1.TracePipeline
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := 0; i < maxNumberOfTracePipelines; i++ {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				pipeline := kitk8s.NewTracePipelineV1Alpha1(pipelineName)
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
				verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
				verifiers.TraceCollectorConfigShouldContainPipeline(ctx, k8sClient, pipelineName)
			}
		})

		It("Should set ConfigurationGenerated condition to false and Pending condition to true", func() {
			By("Creating an additional pipeline", func() {
				pipelineName := suite.IDWithSuffix("limit-exceeding")
				pipeline := kitk8s.NewTracePipelineV1Alpha1(pipelineName)
				pipelineCreatedLater = pipeline.K8sObject()
				pipelinesNames = append(pipelinesNames, pipelineName)

				Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline.K8sObject())).Should(Succeed())

				verifiers.TraceCollectorConfigShouldNotContainPipeline(ctx, k8sClient, pipelineName)

				var fetched telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: pipelineName}
				Expect(k8sClient.Get(ctx, key, &fetched)).To(Succeed())

				configurationGeneratedCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeConfigurationGenerated)
				Expect(configurationGeneratedCond).NotTo(BeNil())
				Expect(configurationGeneratedCond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(configurationGeneratedCond.Reason).Should(Equal(conditions.ReasonMaxPipelinesExceeded))

				pendingCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypePending)
				Expect(pendingCond).NotTo(BeNil())
				Expect(pendingCond.Status).Should(Equal(metav1.ConditionTrue))
				Expect(pendingCond.Reason).Should(Equal(conditions.ReasonMaxPipelinesExceeded))

				runningCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeRunning)
				Expect(runningCond).To(BeNil())
			})
		})

		It("Should have only running pipelines", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineCreatedFirst)).Should(Succeed())

				for _, pipeline := range pipelinesNames[1:] {
					verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipeline)
				}
			})
		})
	})

})
