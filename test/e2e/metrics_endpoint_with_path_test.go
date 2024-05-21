//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	const (
		path     = "/v1/mock"
		endpoint = "metric-mock"
	)

	var endpointDataKey string

	makeResources := func() []client.Object {
		var objs []client.Object

		pipelineName := suite.ID()
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(endpoint), testutils.OTLPEndpointPath(path), testutils.OTLPProtocol("http")).
			Build()

		endpointDataKey = fmt.Sprintf("%s_%s", "OTLP_ENDPOINT", kitkyma.MakeEnvVarCompliant(pipelineName))
		objs = append(objs, &metricPipeline)
		return objs
	}

	Context("When a MetricPipeline with path exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a secret with endpoint and path", func() {
			assert.SecretHasKeyValue(ctx, k8sClient, kitkyma.MetricGatewaySecretName, endpointDataKey, endpoint+path)
		})
	})
})
