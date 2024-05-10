//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	var (
		mockNs           = suite.ID()
		appNs            = "app"
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(), kitk8s.NewNamespace(appNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		backendExportURL = backend.ExportURL(proxyClient)
		objs = append(objs, backend.K8sObjects()...)

		pipelineWithoutOTLP := kitk8s.NewMetricPipelineV1Alpha1(suite.ID()).
			WithOutputEndpointFromSecret(backend.HostSecretRefV1Alpha1()).
			OtlpInput(false)
		objs = append(objs, pipelineWithoutOTLP.K8sObject())

		objs = append(objs, telemetrygen.New(appNs, telemetrygen.SignalTypeMetrics).K8sObject())
		return objs
	}

	Context("When a metricpipeline with disabled OTLP input exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should not deliver OTLP metrics", func() {
			assert.MetricsFromNamespaceShouldNotBeDelivered(proxyClient, backendExportURL, appNs)
		})
	})
})
