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
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics OTLP Input", Label("metrics"), func() {
	const (
		backendNs   = "metric-otlp-input"
		backendName = "backend"
		appNs       = "app"
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject(), kitk8s.NewNamespace(appNs).K8sObject())

		mockBackend := backend.New(backendName, backendNs, backend.SignalTypeMetrics)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)
		objs = append(objs, mockBackend.K8sObjects()...)

		pipelineWithoutOTLP := kitk8s.NewMetricPipeline("pipeline-without-otlp-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			OtlpInput(false)
		objs = append(objs, pipelineWithoutOTLP.K8sObject())

		objs = append(objs, telemetrygen.New(appNs).K8sObject())
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})
		})

		It("Should not deliver OTLP metrics", func() {
			verifiers.MetricsFromNamespaceShouldNotBeDelivered(proxyClient, telemetryExportURL, appNs)
		})
	})
})
