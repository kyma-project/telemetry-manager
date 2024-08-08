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
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), func() {
	const (
		invalidEndpoint = "'http://example.com'"
	)

	var (
		mockNs                = suite.ID()
		pipelineNameValue     = suite.IDWithSuffix("value")
		pipelineNameValueFrom = suite.IDWithSuffix("value-from")
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		logPipelineInvalidEndpointValue := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameValue).
			WithHTTPOutput(
				testutils.HTTPHost(invalidEndpoint),
			).
			Build()

		endpointKey := "endpoint"
		secret := kitk8s.NewOpaqueSecret("test", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, invalidEndpoint))
		logPipelineInvalidEndpointValueFrom := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameValueFrom).
			WithHTTPOutput(testutils.HTTPHostFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()

		objs = append(objs,
			secret.K8sObject(),
			&logPipelineInvalidEndpointValue,
			&logPipelineInvalidEndpointValueFrom,
		)

		return objs
	}

	Context("When log pipelines with an invalid Endpoint are created", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should set ConfigurationGenerated condition to False in pipelines", func() {
			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineNameValue, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointConfigurationInvalid,
			})

			assert.LogPipelineHasCondition(ctx, k8sClient, pipelineNameValueFrom, metav1.Condition{
				Type:   conditions.TypeConfigurationGenerated,
				Status: metav1.ConditionFalse,
				Reason: conditions.ReasonEndpointConfigurationInvalid,
			})
		})
	})
})
