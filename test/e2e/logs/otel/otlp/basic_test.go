//go:build e2e

package otel

import (
	"io"
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
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers/log"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogsOtel, suite.LabelSignalPush, suite.LabelExperimental), Ordered, func() {
	var (
		mockNs           = suite.ID()
		backendNs        = suite.IDWithSuffix("backend")
		pipelineName     = suite.ID()
		backendExportURL string
	)

	makeResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())
		objs = append(objs, kitk8s.NewNamespace(backendNs).K8sObject())

		backend := backend.New(backendNs, backend.SignalTypeLogsOtel, backend.WithPersistentHostSecret(suite.IsUpgrade()))
		objs = append(objs, backend.K8sObjects()...)
		backendExportURL = backend.ExportURL(suite.ProxyClient)

		hostSecretRef := backend.HostSecretRefV1Alpha1()
		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineName).
			WithApplicationInput(false).
			WithKeepOriginalBody(false).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)
		if suite.IsUpgrade() {
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
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(suite.Ctx, suite.K8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running log gateway deployment", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, kitkyma.LogGatewayName)
		})

		It("Should have 2 log gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.LogGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a log backend running", func() {
			assert.DeploymentReady(suite.Ctx, suite.K8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: backendNs})
		})

		It("Should have a running pipeline", func() {
			assert.LogPipelineOtelHealthy(suite.Ctx, suite.K8sClient, pipelineName)
		})

		It("Should deliver telemetrygen logs", func() {
			assert.OtelLogsFromNamespaceDelivered(suite.ProxyClient, backendExportURL, mockNs)
		})

		It("Should have Observed timestamp in the logs", func() {
			Consistently(func(g Gomega) {
				resp, err := suite.ProxyClient.Get(backendExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))

				bodyContent, err := io.ReadAll(resp.Body)
				defer resp.Body.Close()
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(bodyContent).To(HaveFlatOtelLogs(ContainElement(SatisfyAll(
					HaveOtelTimestamp(Not(BeEmpty())),
					HaveObservedTimestamp(Not(Equal("1970-01-01 00:00:00 +0000 UTC")))))))
			}, periodic.ConsistentlyTimeout, periodic.TelemetryInterval).Should(Succeed())
		})

		It("Should be able to get log gateway metrics endpoint", func() {
			gatewayMetricsURL := suite.ProxyClient.ProxyURLForService(kitkyma.LogGatewayMetricsService.Namespace, kitkyma.LogGatewayMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(suite.ProxyClient, gatewayMetricsURL)
		})

		It("Should have a working network policy", func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.LogGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(suite.K8sClient.List(suite.Ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.LogGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				logGatewayPodName := podList.Items[0].Name
				pprofEndpoint := suite.ProxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, logGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := suite.ProxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
