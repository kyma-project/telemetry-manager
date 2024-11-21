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

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/loggen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var (
		mockNsFluentBit       = suite.IDWithSuffix("fluentbit")
		pipelineNameFluentBit = suite.IDWithSuffix("fluentbit")

		mockNsOtel       = suite.IDWithSuffix("otel")
		pipelineNameOtel = suite.IDWithSuffix("otel")

		backendExportURL string
	)

	makeFluentBitResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNsFluentBit).K8sObject())

		backend := backend.New(mockNsFluentBit, backend.SignalTypeLogs, backend.WithPersistentHostSecret(suite.IsOperational()))
		logProducer := loggen.New(mockNsFluentBit)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(proxyClient)
		hostSecretRef := backend.HostSecretRefV1Alpha1()

		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameFluentBit).
			WithHTTPOutput(
				testutils.HTTPHostFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
				testutils.HTTPPort(backend.Port()),
			)
		if suite.IsOperational() {
			pipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		logPipeline := pipelineBuilder.Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	makeOtelResources := func() []client.Object {
		var objs []client.Object
		objs = append(objs, kitk8s.NewNamespace(mockNsOtel).K8sObject())

		backend := backend.New(mockNsOtel, backend.SignalTypeLogs, backend.WithPersistentHostSecret(suite.IsOperational()))
		logProducer := loggen.New(mockNsOtel)
		objs = append(objs, backend.K8sObjects()...)
		objs = append(objs, logProducer.K8sObject())
		backendExportURL = backend.ExportURL(proxyClient)
		hostSecretRef := backend.HostSecretRefV1Alpha1()

		pipelineBuilder := testutils.NewLogPipelineBuilder().
			WithName(pipelineNameOtel).
			WithOTLPOutput(
				testutils.OTLPEndpointFromSecret(
					hostSecretRef.Name,
					hostSecretRef.Namespace,
					hostSecretRef.Key,
				),
			)
		if suite.IsOperational() {
			pipelineBuilder.WithLabels(kitk8s.PersistentLabel)
		}
		logPipeline := pipelineBuilder.Build()
		objs = append(objs, &logPipeline)

		return objs
	}

	Context("Before deploying a logpipeline", func() {
		It("Should have a healthy webhook", func() {
			assert.WebhookHealthy(ctx, k8sClient)
		})
	})

	Context("When a logpipeline with HTTP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeFluentBitResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running pipeline", Label(suite.LabelOperational), func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineNameFluentBit)
		})

		It("Should have running log agent", func() {
			assert.DaemonSetReady(ctx, k8sClient, kitkyma.FluentBitDaemonSetName)
		})

		It("Should have unsupportedMode set to false", func() {
			assert.LogPipelineUnsupportedMode(ctx, k8sClient, pipelineNameFluentBit, false)
		})

		It("Should have a log backend running", Label(suite.LabelOperational), func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNsFluentBit, Name: backend.DefaultName})
		})

		It("Should have a log producer running", Label(suite.LabelOperational), func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Namespace: mockNsFluentBit, Name: loggen.DefaultName})
		})

		It("Should have produced logs in the backend", Label(suite.LabelOperational), func() {
			assert.LogsDelivered(proxyClient, loggen.DefaultName, backendExportURL)
		})
	})

	Context("When a logpipeline with OTLP output exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeOtelResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running log gateway deployment", Label(suite.LabelOperational), func() {
			assert.DeploymentReady(ctx, k8sClient, kitkyma.LogGatewayName)
		})

		It("Should have 2 log gateway replicas", Label(suite.LabelOperational), func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, kitkyma.LogGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Equal(int32(2)))
		})

		It("Should have a log backend running", Label(suite.LabelOperational), func() {
			assert.DeploymentReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNsOtel})
		})

		It("Should have a running pipeline", Label(suite.LabelOperational), func() {
			assert.LogPipelineHealthy(ctx, k8sClient, pipelineNameOtel)
		})

		It("Should deliver telemetrygen logs", Label(suite.LabelOperational), func() {
			assert.LogsFromNamespaceDelivered(proxyClient, backendExportURL, mockNsOtel)
		})

		It("Should be able to get log gateway metrics endpoint", Label(suite.LabelOperational), func() {
			gatewayMetricsURL := proxyClient.ProxyURLForService(kitkyma.LogGatewayMetricsService.Namespace, kitkyma.LogGatewayMetricsService.Name, "metrics", ports.Metrics)
			assert.EmitsOTelCollectorMetrics(proxyClient, gatewayMetricsURL)
		})

		It("Should have a working network policy", Label(suite.LabelOperational), func() {
			var networkPolicy networkingv1.NetworkPolicy
			Expect(k8sClient.Get(ctx, kitkyma.LogGatewayNetworkPolicy, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kitkyma.SystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": kitkyma.LogGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				logGatewayPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kitkyma.SystemNamespaceName, logGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
	})
})
