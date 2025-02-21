//go:build e2e

package metrics

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelMetrics), Label(LabelSetB), Ordered, func() {
	var (
		mockNs           = ID()
		appNs            = "app"
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(), kitk8s.NewNamespace(appNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		backendExportURL = backend.ExportURL(ProxyClient)
		objs = append(objs, backend.K8sObjects()...)

		pipelineWithoutOTLP := testutils.NewMetricPipelineBuilder().
			WithName(ID()).
			WithOTLPInput(false).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint())).
			Build()
		objs = append(objs, &pipelineWithoutOTLP)

		objs = append(objs, telemetrygen.NewPod(appNs, telemetrygen.SignalTypeMetrics).K8sObject())
		return objs
	}

	Context("When a metricpipeline with disabled OTLP input exists", Ordered, func() {
		BeforeAll(func() {
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should not deliver OTLP metrics", func() {
			assert.MetricsFromNamespaceNotDelivered(ProxyClient, backendExportURL, appNs)
		})
	})
})
