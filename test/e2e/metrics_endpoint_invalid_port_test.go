//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), func() {
	const (
		invalidPortEndpoint = "http://example.com:9abc8"
		missingPortEndpoint = "http://example.com:/"
	)

	var (
		mockNs              = suite.ID()
		pipelineNameInvalid = suite.IDWithSuffix("invalid")
		pipelineNameMissing = suite.IDWithSuffix("missing")
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		metricPipelineInvalidPort := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameInvalid).
			WithOTLPOutput(
				testutils.OTLPEndpoint(invalidPortEndpoint),
			).
			Build()

		metricPipelineMissingPort := testutils.NewMetricPipelineBuilder().
			WithName(pipelineNameMissing).
			WithOTLPOutput(
				testutils.OTLPEndpoint(missingPortEndpoint),
			).
			Build()

		objs = append(objs,
			&metricPipelineInvalidPort,
			&metricPipelineMissingPort,
		)

		return objs
	}

	Context("When metric pipelines with an invalid or missing Endpoint port are created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipelines", func() {
			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineNameInvalid, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})

			assert.MetricPipelineHasCondition(ctx, k8sClient, pipelineNameMissing, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointInvalid,
			})
		})
	})
})
