//go:build istio

package istio

import (
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics OTLP Input", Label("metrics"), func() {
	const (
		backendNs            = "istio-metric-otlp-input"
		backendName          = "backend"
		istiofiedBackendNs   = "istio-metric-otlp-input-with-sidecar"
		istiofiedBackendName = "backend-istiofied"

		pushMetricsDepName          = "push-metrics"
		pushMetricsIstiofiedDepName = "push-metrics-istiofied"
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

		metricPipeline := kitk8s.NewMetricPipeline("pipeline-with-otlp-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			OtlpInput(true)
		objs = append(objs, metricPipeline.K8sObject())

		metricPipelineIstiofiedBackend := kitk8s.NewMetricPipeline("pipeline-with-otlp-input-enabled-with-istiofied-backend").
			WithOutputEndpointFromSecret(mockIstiofiedBackend.HostSecretRef()).
			OtlpInput(true)

		objs = append(objs, metricPipelineIstiofiedBackend.K8sObject())

		// set peerauthentication to strict explicitly
		peerAuth := kitk8s.NewPeerAuthentication(istiofiedBackendName, istiofiedBackendNs)
		objs = append(objs, peerAuth.K8sObject(kitk8s.WithLabel("app", istiofiedBackendName)))

		// Create 2 deployments (with and without side-car) which would push the metrics to the metrics gateway.
		podSpec := telemetrygen.PodSpec(telemetrygen.SignalTypeMetrics, "")
		objs = append(objs,
			kitk8s.NewDeployment(pushMetricsDepName, backendNs).WithPodSpec(podSpec).K8sObject(),
			kitk8s.NewDeployment(pushMetricsIstiofiedDepName, istiofiedBackendNs).WithPodSpec(podSpec).K8sObject(),
		)

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
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, backendNs, telemetrygen.MetricNames)
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, istiofiedBackendNs, telemetrygen.MetricNames)

			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryIstiofiedExportURL, backendNs, telemetrygen.MetricNames)
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, telemetryIstiofiedExportURL, istiofiedBackendNs, telemetrygen.MetricNames)

		})
	})
})
