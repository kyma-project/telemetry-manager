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
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Metrics Multi-Pipeline", Label("metrics"), func() {
	Context("When multiple metricpipelines exist", Ordered, func() {
		const (
			mockNs           = "metric-multi-pipeline"
			mockBackendName1 = "metric-receiver-1"
			mockBackendName2 = "metric-receiver-2"
			telemetrygenNs   = "metric-multi-pipeline-test"
		)
		var (
			pipelines         = kitkyma.NewPipelineList()
			backendExportURLs []string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
				kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
			)

			for _, backendName := range []string{mockBackendName1, mockBackendName2} {
				mockBackend := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backendName))
				objs = append(objs, mockBackend.K8sObjects()...)
				backendExportURLs = append(backendExportURLs, mockBackend.ExportURL(proxyClient))

				metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1())
				pipelines.Append(metricPipeline.Name())
				objs = append(objs, metricPipeline.K8sObject())
			}

			objs = append(objs,
				telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
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

		It("Should deliver telemetrygen metrics", func() {
			for _, backendExportURL := range backendExportURLs {
				verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, backendExportURL, telemetrygenNs, telemetrygen.MetricNames)
			}
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
				pipeline := kitk8s.NewMetricPipelineV1Alpha1(fmt.Sprintf("pipeline-%d", i))
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
				pipeline := kitk8s.NewMetricPipelineV1Alpha1("exceeding-pipeline")
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
			telemetrygenNs  = "broken-metric-pipeline-test"
		)
		var (
			healthyPipelineName string
			brokenPipelineName  string
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
				kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
			)

			mockBackend := backend.New(mockNs, backend.SignalTypeMetrics)
			objs = append(objs, mockBackend.K8sObjects()...)
			backendExportURL = mockBackend.ExportURL(proxyClient)

			healthyPipeline := kitk8s.NewMetricPipelineV1Alpha1("healthy").WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1())
			healthyPipelineName = healthyPipeline.Name()
			objs = append(objs, healthyPipeline.K8sObject())

			unreachableHostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
			brokenPipeline := kitk8s.NewMetricPipelineV1Alpha1("broken").WithOutputEndpointFromSecret(unreachableHostSecret.SecretKeyRefV1Alpha1("metric-host"))
			brokenPipelineName = brokenPipeline.Name()
			objs = append(objs, brokenPipeline.K8sObject(), unreachableHostSecret.K8sObject())

			objs = append(objs,
				telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeMetrics).K8sObject(),
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

		It("Should deliver telemetrygen metrics", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, backendExportURL, telemetrygenNs, telemetrygen.MetricNames)
		})
	})
})
