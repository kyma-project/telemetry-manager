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
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	Context("When multiple metricpipelines exist", Ordered, func() {
		var (
			mockNs            = suite.IDWithSuffix("multi-pipeline")
			backend1Name      = suite.IDWithSuffix("backend-1")
			pipeline1Name     = suite.IDWithSuffix("1")
			backend1ExportURL string
			backend2Name      = suite.IDWithSuffix("backend-2")
			pipeline2Name     = suite.IDWithSuffix("2")
			backend2ExportURL string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend1 := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backend1Name))
			objs = append(objs, backend1.K8sObjects()...)
			backend1ExportURL = backend1.ExportURL(proxyClient)

			metricPipeline1 := kitk8s.NewMetricPipelineV1Alpha1(pipeline1Name).WithOutputEndpointFromSecret(backend1.HostSecretRefV1Alpha1())
			objs = append(objs, metricPipeline1.K8sObject())

			backend2 := backend.New(mockNs, backend.SignalTypeMetrics, backend.WithName(backend2Name))
			objs = append(objs, backend2.K8sObjects()...)
			backend2ExportURL = backend2.ExportURL(proxyClient)

			metricPipeline := kitk8s.NewMetricPipelineV1Alpha1(pipeline2Name).WithOutputEndpointFromSecret(backend2.HostSecretRefV1Alpha1())
			objs = append(objs, metricPipeline.K8sObject())

			objs = append(objs,
				telemetrygen.New(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
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
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipeline1Name)
			verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipeline2Name)
		})

		It("Should have a running metric gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.MetricGatewayName)
		})

		It("Should have a metrics backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend1Name, Namespace: mockNs})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend2Name, Namespace: mockNs})
		})

		It("Should deliver telemetrygen metrics", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, backend1ExportURL, mockNs, telemetrygen.MetricNames)
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, backend2ExportURL, mockNs, telemetrygen.MetricNames)
		})
	})

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfMetricPipelines = 3

		var (
			pipelinesNames       = make([]string, 0, maxNumberOfMetricPipelines)
			pipelineCreatedFirst *telemetryv1alpha1.MetricPipeline
			pipelineCreatedLater *telemetryv1alpha1.MetricPipeline
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := 0; i < maxNumberOfMetricPipelines; i++ {
				pipelineName := fmt.Sprintf("%s-limit-%d", suite.ID(), i)
				pipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName)
				pipelinesNames = append(pipelinesNames, pipelineName)

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
			for _, pipelineName := range pipelinesNames {
				verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipelineName)
				verifiers.MetricGatewayConfigShouldContainPipeline(ctx, k8sClient, pipelineName)
			}
		})

		It("Should set ConfigurationGenerated condition to false", func() {
			By("Creating an additional pipeline", func() {
				pipelineName := fmt.Sprintf("%s-limit-exceeding", suite.ID())
				pipeline := kitk8s.NewMetricPipelineV1Alpha1(pipelineName)
				pipelineCreatedLater = pipeline.K8sObject()
				pipelinesNames = append(pipelinesNames, pipelineName)

				Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline.K8sObject())).Should(Succeed())
				Eventually(func(g Gomega) {
					var fetched telemetryv1alpha1.MetricPipeline
					key := types.NamespacedName{Name: pipelineName}
					g.Expect(k8sClient.Get(ctx, key, &fetched)).To(Succeed())
					configurationGeneratedCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeConfigurationGenerated)
					g.Expect(configurationGeneratedCond).NotTo(BeNil())
					g.Expect(configurationGeneratedCond.Status).Should(Equal(metav1.ConditionFalse))
					g.Expect(configurationGeneratedCond.Reason).Should(Equal(conditions.ReasonMaxPipelinesExceeded))
				}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
				verifiers.MetricGatewayConfigShouldNotContainPipeline(ctx, k8sClient, pipelineName)
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineCreatedFirst)).Should(Succeed())

				for _, pipeline := range pipelinesNames[1:] {
					verifiers.MetricPipelineShouldBeHealthy(ctx, k8sClient, pipeline)
				}
			})
		})
	})

	Context("When a broken metricpipeline exists", Ordered, func() {
		var (
			mockNs              = suite.IDWithSuffix("broken-pipeline")
			healthyPipelineName = suite.IDWithSuffix("healthy")
			brokenPipelineName  = suite.IDWithSuffix("broken")
			backendExportURL    string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			backend := backend.New(mockNs, backend.SignalTypeMetrics)
			objs = append(objs, backend.K8sObjects()...)
			backendExportURL = backend.ExportURL(proxyClient)

			healthyPipeline := kitk8s.NewMetricPipelineV1Alpha1(healthyPipelineName).WithOutputEndpointFromSecret(backend.HostSecretRefV1Alpha1())
			objs = append(objs, healthyPipeline.K8sObject())

			unreachableHostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
			brokenPipeline := kitk8s.NewMetricPipelineV1Alpha1(brokenPipelineName).WithOutputEndpointFromSecret(unreachableHostSecret.SecretKeyRefV1Alpha1("metric-host"))
			objs = append(objs, brokenPipeline.K8sObject(), unreachableHostSecret.K8sObject())

			objs = append(objs,
				telemetrygen.New(mockNs, telemetrygen.SignalTypeMetrics).K8sObject(),
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
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: backend.DefaultName, Namespace: mockNs})
		})

		It("Should deliver telemetrygen metrics", func() {
			verifiers.MetricsFromNamespaceShouldBeDelivered(proxyClient, backendExportURL, mockNs, telemetrygen.MetricNames)
		})
	})
})
