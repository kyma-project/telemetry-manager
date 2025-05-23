//go:build e2e

package traces

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

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	var (
		mockNs           = suite.ID()
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object

		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := kitbackend.New(mockNs, kitbackend.SignalTypeTraces, kitbackend.WithPersistentHostSecret(suite.IsUpgrade()))
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		tracePipelineBuilder := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(
				hostSecretRef.Name,
				hostSecretRef.Namespace,
				hostSecretRef.Key,
			))
		if suite.IsUpgrade() {
			tracePipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		tracePipeline := tracePipelineBuilder.Build()

		objs = append(objs,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeTraces).K8sObject(),
			&tracePipeline,
		)
		return objs
	}

	Context("When a tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(suite.Ctx, kitkyma.TraceGatewayName)
		})

		It("Should have 2 trace gateway replicas", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TraceGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should reject scaling below minimum", Label(suite.LabelUpgrade), func() {
			var telemetry operatorv1alpha1.Telemetry
			err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
			Expect(err).NotTo(HaveOccurred())

			telemetry.Spec.Trace = &operatorv1alpha1.TraceSpec{
				Gateway: operatorv1alpha1.TraceGatewaySpec{
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

		It("Should scale up trace gateway replicas", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Trace = &operatorv1alpha1.TraceSpec{
					Gateway: operatorv1alpha1.TraceGatewaySpec{
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
				return telemetry.Spec.Trace.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should have 4 trace gateway replicas after scaling up", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TraceGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(4)))
		})

		It("Should scale down trace gateway replicas", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var telemetry operatorv1alpha1.Telemetry
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TelemetryName, &telemetry)
				g.Expect(err).NotTo(HaveOccurred())

				telemetry.Spec.Trace = &operatorv1alpha1.TraceSpec{
					Gateway: operatorv1alpha1.TraceGatewaySpec{
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
				return telemetry.Spec.Trace.Gateway.Scaling.Static.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have 2 trace gateway replicas after scaling down", Label(suite.LabelUpgrade), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.TraceGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a trace backend running", Label(suite.LabelUpgrade), func() {
			assert.DeploymentReady(suite.Ctx, types.NamespacedName{Name: kitbackend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", Label(suite.LabelUpgrade), func() {
			assert.TracePipelineHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should deliver telemetrygen traces", Label(suite.LabelUpgrade), func() {
			assert.TracesFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})

		It("Should be able to get trace gateway metrics endpoint", Label(suite.LabelUpgrade), func() {
			gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.TraceGatewayMetricsService.Namespace, kitkyma.TraceGatewayMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(suite.Ctx, gatewayMetricsURL)
		})

		It("Should have a working network policy", Label(suite.LabelUpgrade), func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.TraceGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(suite.K8sClient.List(suite.Ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.TraceGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				traceGatewayPodName := podList.Items[0].Name
				pprofEndpoint := suite.ProxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, traceGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := suite.ProxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
