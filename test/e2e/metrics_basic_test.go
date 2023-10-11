//go:build e2e

package e2e

import (
	"fmt"
	"net/http"

	"github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Basic", Label("metrics"), func() {
	const (
		mockBackendName = "metric-receiver"
		mockNs          = "metric-mocks"
	)

	var (
		pipelineName string
		urls         = urlprovider.New()
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics, backend.WithPersistentHostSecret(isOperational()))
		objs = append(objs, mockBackend.K8sObjects()...)
		urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

		metricPipeline := kitmetric.NewPipeline(fmt.Sprintf("%s-pipeline", mockBackend.Name())).
			WithOutputEndpointFromSecret(mockBackend.HostSecretRef()).
			Persistent(isOperational())
		pipelineName = metricPipeline.Name()
		objs = append(objs, metricPipeline.K8sObject())

		urls.SetOTLPPush(proxyClient.ProxyURLForService(
			kitkyma.SystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP),
		)

		metricGatewayExternalService := kitk8s.NewService("telemetry-otlp-metrics-external", kitkyma.SystemNamespaceName).
			WithPort("grpc-otlp", ports.OTLPGRPC).
			WithPort("http-metrics", ports.Metrics)
		urls.SetMetrics(proxyClient.ProxyURLForService(
			kitkyma.SystemNamespaceName, "telemetry-otlp-metrics-external", "metrics", ports.Metrics),
		)

		objs = append(objs, metricGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", kitkyma.MetricGatewayBaseName)))
		return objs
	}

	Context("When a metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", Label(operationalTest), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should scale up trace metric replicas", Label(operationalTest), func() {
			Eventually(func(g Gomega) int32 {
				var telemetry v1alpha1.Telemetry
				err := k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Metric.Gateway.Scaling = v1alpha1.Scaling{
					Type: v1alpha1.StaticScalingStrategyType,
					Static: &v1alpha1.StaticScaling{
						Replicas: 4,
					},
				}
				err = k8sClient.Update(ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should have 4 metric gateway replicas after scaling up", Label(operationalTest), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should scale down metric gateway replicas", Label(operationalTest), func() {
			Eventually(func(g Gomega) int32 {
				var telemetry v1alpha1.Telemetry
				err := k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Metric.Gateway.Scaling = v1alpha1.Scaling{
					Type: v1alpha1.StaticScalingStrategyType,
					Static: &v1alpha1.StaticScaling{
						Replicas: 2,
					},
				}
				err = k8sClient.Update(ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have 2 metric gateway replicas after scaling down", Label(operationalTest), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a metrics backend running", Label(operationalTest), func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should be able to get metric gateway metrics endpoint", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.Metrics())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have a running pipeline", Label(operationalTest), func() {
			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipelineName)
		})

		It("Should verify end-to-end metric delivery", Label(operationalTest), func() {
			gauges := kitmetrics.MakeAndSendGaugeMetrics(proxyClient, urls.OTLPPush())
			verifiers.MetricsShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), gauges)
		})

		It("Should have a working network policy", Label(operationalTest), func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(k8sClient.Get(ctx, kitkyma.MetricGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.MetricGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				metricGatewayPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, metricGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
