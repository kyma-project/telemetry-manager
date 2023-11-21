//go:build istio

package istio

import (
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/metricproducer"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Istio Input", Label("metrics"), func() {
	const (
		mockNs             = "metric-istio-input"
		mockBackendName    = "metric-agent-receiver"
		metricProducerName = "metric-producer-with-proxy"
	)
	var telemetryExportURL string

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		// Mocks namespace objects
		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
		objs = append(objs, mockBackend.K8sObjects()...)
		telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

		metricProducer := metricproducer.New(mockNs, metricproducer.WithName(metricProducerName))
		objs = append(objs, []client.Object{
			metricProducer.Pod().WithSidecarInjection().K8sObject(),
		}...)

		metricPipeline := kitmetric.NewPipeline("pipeline-with-istio-input-enabled").
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			IstioInput(true)
		objs = append(objs, metricPipeline.K8sObject())

		return objs
	}

	Context("App with istio-sidecar", Ordered, func() {
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should have a running metric agent daemonset", func() {
			verifiers.DaemonSetShouldBeReady(ctx, k8sClient, kitkyma.MetricAgentName)
		})

		It("Should verify istio proxy metric scraping", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(telemetryExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				// here we are discovering the same metric-producer workload twice: once via the annotated service and once via the annotated pod
				// targets discovered via annotated pods must have no service label
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUTemperature.Name)),
						WithType(Equal(metricproducer.MetricCPUTemperature.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricCPUEnergyHistogram.Name)),
						WithType(Equal(metricproducer.MetricCPUEnergyHistogram.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardwareHumidity.Name)),
						WithType(Equal(metricproducer.MetricHardwareHumidity.Type)),
					))),
					ContainMd(ContainMetric(SatisfyAll(
						WithName(Equal(metricproducer.MetricHardDiskErrorsTotal.Name)),
						WithType(Equal(metricproducer.MetricHardDiskErrorsTotal.Type)),
					))),
				),
				))
			}, periodic.TelemetryEventuallyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})
	})
})
