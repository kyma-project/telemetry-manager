//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetricpipeline "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Endpoint with Path", Label("metrics"), func() {
	const (
		path                    = "/v1/mock"
		endpoint                = "metric-mock"
		endpointEndpointDataKey = "OTLP_ENDPOINT_MOCK_METRIC_ENDPOINT_PATH"
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		metricPipeline := kitmetricpipeline.NewPipeline("mock-metric-endpoint-path").
			WithProtocol("http").
			WithOutputEndpoint(endpoint).WithEndpointPath(path).
			Persistent(true)
		objs = append(objs, metricPipeline.K8sObject())
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
			verifiers.SecretShouldHaveValue(ctx, k8sClient, kitkyma.MetricGatewaySecretName, endpointEndpointDataKey, endpoint+path)
		})
	})
})
