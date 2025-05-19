//go:build e2e

package logs

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMaxPipeline, suite.LabelLogs, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs = suite.ID()
	)

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfLogPipelines = telemetrycontrollers.MaxPipelineCount

		var (
			backend       = backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithPersistentHostSecret(suite.IsUpgrade()))
			hostSecretRef = backend.HostSecretRefV1Alpha1()

			pipelinesNames = make([]string, 0, maxNumberOfLogPipelines)
			pipelines      []client.Object

			additionalFBPipelineName = fmt.Sprintf("%s-limit-exceeding-fluentbit", suite.ID())
			additionalFBPipeline     = ptr.To(testutils.NewLogPipelineBuilder().
							WithName(additionalFBPipelineName).
							Build())

			additionalOtelPipelineName = fmt.Sprintf("%s-limit-exceeding-otel", suite.ID())
			additionalOtelPipeline     = ptr.To(testutils.NewLogPipelineBuilder().
							WithName(additionalOtelPipelineName).
							WithApplicationInput(false).
							WithKeepOriginalBody(false).
							WithOTLPOutput(
					testutils.OTLPEndpointFromSecret(
						hostSecretRef.Name,
						hostSecretRef.Namespace,
						hostSecretRef.Key,
					),
				).
				Build())
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := range maxNumberOfLogPipelines {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				// every other pipeline will have a will have HTTP output
				var pipeline telemetryv1alpha1.LogPipeline
				if i%2 == 0 {
					pipeline = testutils.NewLogPipelineBuilder().
						WithName(pipelineName).
						WithHTTPOutput().
						Build()
				} else {
					pipeline = testutils.NewLogPipelineBuilder().
						WithName(pipelineName).
						WithApplicationInput(false).
						WithKeepOriginalBody(false).
						WithOTLPOutput(
							testutils.OTLPEndpointFromSecret(
								hostSecretRef.Name,
								hostSecretRef.Namespace,
								hostSecretRef.Key,
							),
						).Build()
				}
				pipelines = append(pipelines, &pipeline)
				pipelinesNames = append(pipelinesNames, pipelineName)
			}
			objs = append(objs, pipelines...)

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, additionalFBPipeline)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, additionalOtelPipeline)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects[2:]...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipelineName := range pipelinesNames {
				assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
			}
		})
		It("Should create additional pipeline in not healthy state", func() {
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, additionalFBPipeline)).Should(Succeed())

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalFBPipeline.GetName(), metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonMaxPipelinesExceeded,
			})

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalFBPipeline.GetName(), metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should delete one previously healthy pipeline and render the additional pipeline healthy", func() {
			deletePipeline := pipelines[0]
			Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, deletePipeline)).Should(Succeed())
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, additionalFBPipeline.GetName())
		})

		It("Should create additional pipeline in not healthy state", func() {
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, additionalOtelPipeline)).Should(Succeed())

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalOtelPipeline.GetName(), metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonMaxPipelinesExceeded,
			})

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalOtelPipeline.GetName(), metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should delete one previously healthy pipeline and render the additional pipeline healthy", func() {
			deletePipeline := pipelines[1]
			Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, deletePipeline)).Should(Succeed())
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, additionalOtelPipeline.GetName())
		})
	})
})
