//go:build e2e

package otel

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetrycontrollers "github.com/kyma-project/telemetry-manager/controllers/telemetry"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelExperimental, suite.LabelMaxPipeline), Ordered, func() {
	var (
		mockNs = suite.ID()
	)

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfLogPipelines = telemetrycontrollers.MaxPipelineCount

		var (
			pipelines          []client.Object
			additionalPipeline client.Object
		)

		backend := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithPersistentHostSecret(suite.IsUpgrade()))
		hostSecretRef := backend.HostSecretRefV1Alpha1()
		makeResources := func() []client.Object {
			var objs []client.Object
			for i := range maxNumberOfLogPipelines {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				pipeline := testutils.NewLogPipelineBuilder().
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
				pipelines = append(pipelines, &pipeline)
			}
			objs = append(objs, pipelines...)

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipeline := range pipelines {
				assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipeline.GetName())
			}
		})

		It("Should create an additional pipeline in not healthy state", func() {
			additionalPipelineName := fmt.Sprintf("%s-limit-exceeding", suite.ID())
			additionalPipeline = ptr.To(testutils.NewLogPipelineBuilder().WithName(additionalPipelineName).Build())

			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, additionalPipeline)).Should(Succeed())

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalPipeline.GetName(), metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonMaxPipelinesExceeded,
			})

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalPipeline.GetName(), metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should delete one previously healthy pipeline and render the additional pipeline healthy", func() {
			var deletePipeline client.Object
			deletePipeline, pipelines = pipelines[0], pipelines[1:]
			Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, deletePipeline)).Should(Succeed())
			assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, additionalPipeline.GetName())
		})

	})
})
