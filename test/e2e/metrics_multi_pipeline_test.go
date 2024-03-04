//go:build e2e

package e2e

import (
	"fmt"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otel/metrics"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Multi-Pipeline", Label("metrics"), func() {
	Context("When multiple metricpipelines exist", Ordered, func() {
		const (
			mockNs           = "metric-multi-pipeline"
			mockBackendName1 = "metric-receiver-1"
			mockBackendName2 = "metric-receiver-2"
		)
		var (
			pipelines = kitkyma.NewPipelineList()
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

				metricPipeline := kitk8s.NewMetricPipeline(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
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
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelines.First())
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelines.Second())
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
			pipelines            = kitkyma.NewPipelineList()
			pipelineCreatedFirst *telemetryv1alpha1.MetricPipeline
			pipelineCreatedLater *telemetryv1alpha1.MetricPipeline
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := 0; i < maxNumberOfMetricPipelines; i++ {
				pipeline := kitk8s.NewMetricPipeline(fmt.Sprintf("pipeline-%d", i))
				pipelines.Append(pipeline.Name())
				objs = append(objs, pipeline.K8sObject())

				if i == 0 {
					pipelineCreatedFirst = pipeline.K8sObject()
				}
			}

			return objs
		}

		BeforeAll(func() {
			k8sObjects := makeResources()
			DeferCleanup(func() {
				k8sObjectsToDelete := slices.DeleteFunc(k8sObjects, func(obj client.Object) bool {
					return obj.GetName() == pipelineCreatedFirst.GetName() //first pipeline is deleted separately in one of the specs
				})
				k8sObjectsToDelete = append(k8sObjectsToDelete, pipelineCreatedLater)
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjectsToDelete...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have only running pipelines", func() {
			for _, pipeline := range pipelines.All() {
				verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipeline)
				verifiers.MetricGatewayConfigShouldContainPipeline(ctx, k8sClient, pipeline)
			}
		})

		It("Should set ConfigurationGenerated condition to false", func() {
			By("Creating an additional pipeline", func() {
				pipeline := kitk8s.NewMetricPipeline("exceeding-pipeline")
				pipelineCreatedLater = pipeline.K8sObject()
				pipelines.Append(pipeline.Name())

				Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline.K8sObject())).Should(Succeed())
				Eventually(func(g Gomega) {
					var fetched telemetryv1alpha1.MetricPipeline
					key := types.NamespacedName{Name: pipeline.Name()}
					g.Expect(k8sClient.Get(ctx, key, &fetched)).To(Succeed())
					configurationGeneratedCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeConfigurationGenerated)
					g.Expect(configurationGeneratedCond).NotTo(BeNil())
					g.Expect(configurationGeneratedCond.Status).Should(Equal(metav1.ConditionFalse))
					g.Expect(configurationGeneratedCond.Reason).Should(Equal(conditions.ReasonMaxPipelinesExceeded))
				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
				verifiers.MetricGatewayConfigShouldNotContainPipeline(ctx, k8sClient, pipeline.Name())
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineCreatedFirst)).Should(Succeed())

				for _, pipeline := range pipelines.All()[1:] {
					verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipeline)
				}
			})
		})
	})

	Context("When a broken metricpipeline exists", Ordered, func() {
		const (
			mockBackendName = "metric-receiver"
			mockNs          = "metric-mocks-broken-pipeline"
		)
		var (
			urls                = urlprovider.New()
			healthyPipelineName string
			brokenPipelineName  string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeMetrics)
			objs = append(objs, mockBackend.K8sObjects()...)
			urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

			healthyPipeline := kitk8s.NewMetricPipeline("healthy").WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
			healthyPipelineName = healthyPipeline.Name()
			objs = append(objs, healthyPipeline.K8sObject())

			unreachableHostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
			brokenPipeline := kitk8s.NewMetricPipeline("broken").WithOutputEndpointFromSecret(unreachableHostSecret.SecretKeyRef("metric-host"))
			brokenPipelineName = brokenPipeline.Name()
			objs = append(objs, brokenPipeline.K8sObject(), unreachableHostSecret.K8sObject())

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
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, healthyPipelineName)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, brokenPipelineName)
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := kitmetrics.MakeAndSendGaugeMetrics(proxyClient, urls.OTLPPush())
			verifiers.MetricsShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), gauges)
		})
	})
})
