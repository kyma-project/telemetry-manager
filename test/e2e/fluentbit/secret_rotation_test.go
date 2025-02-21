//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	var pipelineName = ID()

	Context("When a LogPipeline with missing secret reference exists", Ordered, func() {

		endpointKey := "logs-endpoint"
		secret := kitk8s.NewOpaqueSecret("logs-missing", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		logPipeline := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithHTTPOutput(testutils.HTTPHostFromSecret(
				secret.Name(),
				secret.Namespace(),
				endpointKey,
			)).
			Build()

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &logPipeline)).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, &logPipeline)).Should(Succeed())
			})
		})

		It("Should set ConfigurationGenerated condition to False in pipeline", func() {
			assert.LogPipelineHasCondition(Ctx, K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})

		It("Should set TelemetryFlowHealthy condition to False in pipeline", func() {
			assert.LogPipelineHasCondition(Ctx, K8sClient, pipelineName, metav1.Condition{
				Type:   conditions.TypeFlowHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonSelfMonConfigNotGenerated,
			})
		})

		It("Should set LogComponentsHealthy condition to False in Telemetry", func() {
			assert.TelemetryHasState(Ctx, K8sClient, operatorv1alpha1.StateWarning)
			assert.TelemetryHasCondition(Ctx, K8sClient, metav1.Condition{
				Type:   conditions.TypeLogComponentsHealthy,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonReferencedSecretMissing,
			})
		})

		It("Should have a healthy pipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(Ctx, K8sClient, secret.K8sObject())).Should(Succeed())
			})

			assert.LogPipelineHealthy(Ctx, K8sClient, pipelineName)
		})
	})
})
