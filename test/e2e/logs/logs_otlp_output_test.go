//go:build e2e

package logs

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs, LabelExperimental), Ordered, func() {
	var (
		mockNs           = ID()
		pipelineName     = ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

		backend := backend.New(mockNs, backend.SignalTypeLogsOtel, backend.WithPersistentHostSecret(IsUpgrade()))
		logProducer := loggen.New(mockNs)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)
		if IsUpgrade() {
			pipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		logPipeline := pipelineBuilder.Build()

		objs = append(objs,
			telemetrygen.NewPod(mockNs, telemetrygen.SignalTypeLogs).K8sObject(),
			&logPipeline,
		)
		return objs
	}

	Context("When a logpipeline with OTLP output exists", Ordered, func() {
		BeforeAll(func() {
			K8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, K8sObjects...)).Should(Succeed())
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(Ctx, K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have 2 log gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := K8sClient.Get(Ctx, kitkyma.LogGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(Ctx, K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineOtelHealthy(Ctx, K8sClient, pipelineName)
		})

		It("Should deliver telemetrygen logs", func() {
			assert.LogsFromNamespaceDelivered(ProxyClient, backendExportURL, mockNs)
		})

		It("Should be able to get log gateway metrics endpoint", func() {
			gatewayMetricsURL := ProxyClient.ProxyURLForService(kitkyma.LogGatewayMetricsService.Namespace, kitkyma.LogGatewayMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(ProxyClient, gatewayMetricsURL)
		})

		It("Should have a working network policy", func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(K8sClient.Get(Ctx, kitkyma.LogGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(K8sClient.List(Ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.LogGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				logGatewayPodName := podList.Items[0].Name
				pprofEndpoint := ProxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, logGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := ProxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
