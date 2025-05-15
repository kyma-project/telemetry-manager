//go:build e2e

package fluentbit

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/controllers/telemetry"
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
		const maxNumberOfLogPipelines = telemetry.MaxPipelineCount

		var (
			pipelinesNames = make([]string, 0, maxNumberOfLogPipelines)
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
				pipelinesNames = append(pipelinesNames, pipelineName)

				objs = append(objs, &pipeline)
			}

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
			for _, pipelineName := range pipelinesNames {
				assert.LogPipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
			}
		})
		additionalPipelineName := fmt.Sprintf("%s-limit-exceeding", suite.ID())
		var pipeline telemetryv1alpha1.LogPipeline

		It("Should create an additional pipeline in not healthy state", func() {
			pipeline = testutils.NewLogPipelineBuilder().WithName(additionalPipelineName).Build()

			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, &pipeline)).Should(Succeed())

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalPipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonMaxPipelinesExceeded,
			})

			assert.LogPipelineHasCondition(suite.Ctx, suite.K8sClient, additionalPipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})
	})
})
