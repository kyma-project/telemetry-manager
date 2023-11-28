//go:build istio

package istio

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma/istio"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics OTLP Input", Label("metrics"), func() {
	const (
		backendNs            = "istio-metric-otlp-input"
		backendName          = "backend"
		istiofiedBackendNs   = "istio-metric-otlp-input-with-sidecar"
		istiofiedBackendName = "backend-istiofied"
	)
	var telemetryExportURL, telemetryIstiofiedExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(istiofiedBackendNs, kitk8s.WithIstioInjection()).K8sObject())

		// Mocks namespace objects
		mockBackend := backend.New(backendName, backendNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		mockIstiofiedBackend := backend.New(istiofiedBackendName, istiofiedBackendNs, backend.SignalTypeMetrics)
		objs = append(objs, mockIstiofiedBackend.K8sObjects()...)
		telemetryIstiofiedExportURL = mockIstiofiedBackend.TelemetryExportURL(proxyClient)

		metricPipeline := kitmetric.NewPipeline("pipeline-with-otlp-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			OtlpInput(true)
		objs = append(objs, metricPipeline.K8sObject())

		metricPipelineIstiofiedBackend := kitmetric.NewPipeline("pipeline-with-otlp-input-enabled-with-istiofied-backend").
			WithOutputEndpointFromSecret(mockIstiofiedBackend.HostSecretRef()).
			OtlpInput(true)

		objs = append(objs, metricPipelineIstiofiedBackend.K8sObject())

		// set peerauthentication to strict explicitly
		peerAuth := istio.NewPeerAuthentication(istiofiedBackendName, istiofiedBackendNs)
		objs = append(objs, peerAuth.K8sObject(kitk8s.WithLabel("app", istiofiedBackendName)))

		return objs
	}

	Context("Istiofied and non-istiofied in-cluster backends", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				verifiers.ShouldNotExist(ctx, k8sClient, k8sObjects...)
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backendName, Namespace: backendNs})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: istiofiedBackendName, Namespace: istiofiedBackendNs})
		})

		It("Should push metrics successfully", func() {
			gatewayPushURL := proxyClient.ProxyURLForService(kitkyma.SystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP)
			gauges := kitmetrics.MakeAndSendGaugeMetrics(proxyClient, gatewayPushURL)
			pushMetricsShouldBeDelivered(telemetryExportURL, gauges)
			pushMetricsShouldBeDelivered(telemetryIstiofiedExportURL, gauges)
		})
	})
})

func pushMetricsShouldBeDelivered(proxyUrl string, gauges []pmetric.Metric) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyUrl)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(ContainMd(metric.WithMetrics(BeEquivalentTo(gauges)))))
	}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
}
