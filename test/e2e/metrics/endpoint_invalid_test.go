//go:build e2e

package metrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetC), func() {
	const (
		invalidEndpoint = "'http://example.com'"
	)

	var (
		mockNs                = suite.ID()
		pipelineNameValue     = suite.IDWithSuffix("value")
		pipelineNameValueFrom = suite.IDWithSuffix("value-from")
		pipelineNameHTTP      = suite.IDWithSuffix("invalid-http")
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		metricPipelineInvalidEndpointValue := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameValue).
			WithOTLPOutput(
				testutils.OTLPEndpoint(invalidEndpoint),
			).
			Build()

		endpointKey := "endpoint"
		secret := kitk8s.NewOpaqueSecret("test", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, invalidEndpoint))
		metricPipelineInvalidEndpointValueFrom := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameValueFrom).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()

		metricPipelineInvalidEndpointHTTP := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameHTTP).
			WithOTLPOutput(
				testutils.OTLPEndpoint(invalidEndpoint),
				testutils.OTLPProtocol("http"),
			).
			Build()

		objs = append(objs,
			secret.K8sObject(),
			&metricPipelineInvalidEndpointValue,
			&metricPipelineInvalidEndpointValueFrom,
			&metricPipelineInvalidEndpointHTTP,
		)

		return objs
	}

	Context("When metric pipelines with an invalid Endpoint are created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipelines", func() {
			assert.MetricPipelineHasCondition(suite.Ctx, pipelineNameValue, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.MetricPipelineHasCondition(suite.Ctx, pipelineNameValueFrom, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.MetricPipelineHasCondition(suite.Ctx, pipelineNameHTTP, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})
		})
	})
})
