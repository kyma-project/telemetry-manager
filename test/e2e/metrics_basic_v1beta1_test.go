//go:build e2e

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics, suite.LabelV1Beta1), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(proxyClient)

		metricPipeline := kitk8s.NewMetricPipelineV1Beta1(pipelineName).
			WithOutputEndpointFromSecret(backend.HostSecretRefV1Beta1())
		objs = append(objs,
			telemetrygen.New(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			metricPipeline.K8sObject(),
		)

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

		It("Should have a running metric gateway deployment", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should reject scaling below minimum", func() {
			var telemetry operatorv1alpha1.Telemetry
			err := k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)
			Expect(err).NotTo(HaveOccurred())

			telemetry.Spec.Metric = &operatorv1alpha1.MetricSpec{
				Gateway: operatorv1alpha1.MetricGatewaySpec{
					Scaling: operatorv1alpha1.Scaling{
						Type: operatorv1alpha1.StaticScalingStrategyType,
						Static: &operatorv1alpha1.StaticScaling{
							Replicas: -1,
						},
					},
				},
			}
			err = k8sClient.Update(ctx, &telemetry)
			Expect(err).To(HaveOccurred())
		})

		It("Should scale up metric gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Metric = &operatorv1alpha1.MetricSpec{
					Gateway: operatorv1alpha1.MetricGatewaySpec{
						Scaling: operatorv1alpha1.Scaling{
							Type: operatorv1alpha1.StaticScalingStrategyType,
							Static: &operatorv1alpha1.StaticScaling{
								Replicas: 4,
							},
						},
					},
				}
				err = k8sClient.Update(ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should have 4 metric gateway replicas after scaling up", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should scale down metric gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var telemetry operatorv1alpha1.Telemetry
				err := k8sClient.Get(ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Metric = &operatorv1alpha1.MetricSpec{
					Gateway: operatorv1alpha1.MetricGatewaySpec{
						Scaling: operatorv1alpha1.Scaling{
							Type: operatorv1alpha1.StaticScalingStrategyType,
							Static: &operatorv1alpha1.StaticScaling{
								Replicas: 2,
							},
						},
					},
				}
				err = k8sClient.Update(ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have 2 metric gateway replicas after scaling down", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a metrics backend running", func() {
			assert.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
		})

		It("Should deliver telemetrygen metrics", Label(suite.LabelOperational), func() {
			assert.MetricsFromNamespaceShouldBeDelivered(proxyClient, backendExportURL, mockNs, telemetrygen.MetricNames)
		})

		It("Should be able to get metric gateway metrics endpoint", func() {
			gatewayMetricsURL := proxyClient.ProxyURLForService(kitkyma.MetricGatewayMetrics.Namespace, kitkyma.MetricGatewayMetrics.Name, "metrics", ports.Metrics)
			assert.ShouldExposeCollectorMetrics(proxyClient, gatewayMetricsURL)
		})

		It("Should have a working network policy", func() {
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
