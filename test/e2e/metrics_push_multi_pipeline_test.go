//go:build e2e

package e2e

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics", Label("metrics"), func() {
	Context("When multiple metricpipelines exist", Ordered, func() {
		const (
			mockNs           = "metric-mocks-multi-pipeline"
			mockBackendName1 = "metric-receiver-1"
			mockBackendName2 = "metric-receiver-2"
		)
		var (
			pipelines = kyma.NewPipelineList()
			urls      = urlprovider.New()
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			for _, backendName := range []string{mockBackendName1, mockBackendName2} {
				mockBackend := backend.New(backendName, mockNs, backend.SignalTypeMetrics)
				objs = append(objs, mockBackend.K8sObjects()...)
				urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
					mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
				)

				metricPipeline := kitmetric.NewPipeline(
					fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline"),
					mockBackend.HostSecretRefKey(),
				)
				pipelines.Append(metricPipeline.Name())
				objs = append(objs, metricPipeline.K8sObject())
			}

			urls.SetOTLPPush(proxyClient.ProxyURLForService(
				kitkyma.SystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP),
			)
			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipelines.First())
			verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipelines.Second())
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName1, Namespace: mockNs})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName2, Namespace: mockNs})
		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := kitmetrics.MakeAndSendGaugeMetrics(proxyClient, urls.OTLPPush())
			verifiers.MetricsShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName1), gauges)
			verifiers.MetricsShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName2), gauges)
		})
	})

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfMetricPipelines = 3

		var (
			pipelineObjectsToDelete []client.Object
			pipelines               = kyma.NewPipelineList()
		)

		makeResources := func() []client.Object {
			var allObjs []client.Object
			for i := 0; i < maxNumberOfMetricPipelines; i++ {
				name := fmt.Sprintf("pipeline-%d", i)
				hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+name, kitkyma.DefaultNamespaceName,
					kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
				pipeline := kitmetric.NewPipeline(name, hostSecret.SecretKeyRef("metric-host"))
				objs := []client.Object{hostSecret.K8sObject(), pipeline.K8sObject()}
				pipelines.Append(pipeline.Name())
				allObjs = append(allObjs, objs...)
				if i == 0 {
					pipelineObjectsToDelete = objs
				}
			}

			return allObjs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipeline := range pipelines.All() {
				verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipeline)
				verifiers.MetricGatewayConfigShouldContainPipeline(ctx, k8sClient, pipeline)
			}
		})

		It("Should have a pending pipeline", func() {
			By("Creating an additional pipeline", func() {
				hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-exceeding-pipeline", kitkyma.DefaultNamespaceName,
					kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
				pipeline := kitmetric.NewPipeline("exceeding-pipeline", hostSecret.SecretKeyRef("metric-host"))
				newObjs := []client.Object{hostSecret.K8sObject(), pipeline.K8sObject()}
				pipelines.Append(pipeline.Name())

				Expect(kitk8s.CreateObjects(ctx, k8sClient, newObjs...)).Should(Succeed())
				verifiers.MetricPipelineShouldStayPending(ctx, k8sClient, pipeline.Name())
				verifiers.MetricGatewayConfigShouldNotContainPipeline(ctx, k8sClient, pipeline.Name())
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineObjectsToDelete...)).Should(Succeed())
				for _, pipeline := range pipelines.All()[1:] {
					verifiers.MetricPipelineShouldBeRunning(ctx, k8sClient, pipeline)
				}
			})
		})
	})
})
