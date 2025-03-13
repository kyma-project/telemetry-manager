//go:build e2e

package traces

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelTraces), func() {
	const (
		path     = "/v1/mock"
		endpoint = "http://metric-mock:8080"
	)

	var endpointDataKey string

	makeResources := func() []client.Object {
		var objs []client.Object

		pipelineName := ID()
		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpoint(endpoint),
				testutils.OTLPEndpointPath(path),
				testutils.OTLPProtocol("http"),
			).
			Build()

		endpointDataKey = fmt.Sprintf("%s_%s", "OTLP_ENDPOINT", kitkyma.MakeEnvVarCompliant(pipelineName))
		objs = append(objs, &tracePipeline)
		return objs
	}

	Context("When a TracePipeline with path exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a secret with endpoint and path", func() {
			assert.SecretHasKeyValue(Ctx, K8sClient, kitkyma.TraceGatewaySecretName, endpointDataKey, endpoint+path)
		})
	})

})
