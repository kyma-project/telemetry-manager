//go:build e2e

package metrics

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
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitbackend "github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Label(suite.LabelSetB), Ordered, func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeMetrics)
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		metricPipelineBuilder := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpoint(backend.Endpoint()))
		if suite.IsUpgrade() {
			metricPipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		metricPipeline := metricPipelineBuilder.Build()
		objs = append(objs,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
			&metricPipeline,
		)

		return objs
	}

	Context("When a metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.MetricGatewayName)
		})

		It("Should reject scaling below minimum", Label(suite.LabelUpgrade), func() {
			var telemetry operatorv1alpha1.Telemetry
			err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
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
			err = suite.K8sClient.Update(suite.Ctx, &telemetry)
			Expect(err).To(HaveOccurred())
		})

		It("Should scale up metric gateway replicas", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
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
				err = suite.K8sClient.Update(suite.Ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should have 4 metric gateway replicas after scaling up", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.MetricGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should scale down metric gateway replicas", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
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
				err = suite.K8sClient.Update(suite.Ctx, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())
				return telemetry.Spec.Metric.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have 2 metric gateway replicas after scaling down", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.MetricGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a metrics backend running", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
			assert.ServiceReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", Label(suite.LabelUpgrade), func() {
			assert.MetricPipelineHealthy(suite.Ctx, pipelineName)
		})

		It("Should deliver telemetrygen metrics", Label(suite.LabelUpgrade), func() {
			assert.MetricsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs, telemetrygen.MetricNames)
		})

		It("Should be able to get metric gateway metrics endpoint", Label(suite.LabelUpgrade), func() {
			gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.MetricGatewayMetricsService.Namespace, kitkyma.MetricGatewayMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(suite.Ctx, gatewayMetricsURL)
		})

		It("Should have a working network policy", Label(suite.LabelUpgrade), func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.MetricGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(suite.K8sClient.List(suite.Ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.MetricGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				metricGatewayPodName := podList.Items[0].Name
				pprofEndpoint := suite.ProxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, metricGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := suite.ProxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
