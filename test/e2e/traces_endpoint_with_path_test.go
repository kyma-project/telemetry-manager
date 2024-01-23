//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces Endpoint with Path", Label("tracing"), func() {
	const (
		path     = "/v1/mock"
		endpoint = "trace-mock"
	)

	var endpointDataKey string

	makeResources := func() []client.Object {
		var objs []client.Object

		tracePipeline := kitk8s.NewTracePipeline("mock-trace-endpoint-path").
			WithProtocol("http").
			WithOutputEndpoint(endpoint).WithEndpointPath(path)

		pipelineName := tracePipeline.Name()
		endpointDataKey = fmt.Sprintf("%s_%s", "OTLP_ENDPOINT", kitkyma.MakeEnvVarCompliant(pipelineName))
		objs = append(objs, tracePipeline.K8sObject())
		return objs
	}

	Context("When a TracePipeline with path exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a secret with endpoint and path", func() {
			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.TraceGatewaySecretName, endpointDataKey, endpoint+path)
		})
	})

})
