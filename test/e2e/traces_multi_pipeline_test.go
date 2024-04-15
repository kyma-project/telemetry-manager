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
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces Multi-Pipeline", Label("traces"), func() {
	Context("When multiple tracepipelines exist", Ordered, func() {
		const (
			mockNs           = "traces-multi-pipeline"
			mockBackendName1 = "trace-receiver-1"
			mockBackendName2 = "trace-receiver-2"
			telemetrygenNs   = "trace-multi-pipeline-test"
		)
		var (
			pipelines           = kitkyma.NewPipelineList()
			telemetryExportURLs []string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
				kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
			)

			for _, backendName := range []string{mockBackendName1, mockBackendName2} {
				mockBackend := backend.New(backendName, mockNs, backend.SignalTypeTraces)
				objs = append(objs, mockBackend.K8sObjects()...)
				telemetryExportURLs = append(telemetryExportURLs, mockBackend.TelemetryExportURL(proxyClient))

				pipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1())
				pipelines.Append(pipeline.Name())
				objs = append(objs, pipeline.K8sObject())
			}

			objs = append(objs,
				telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeTraces).K8sObject(),
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
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelines.First())
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipelines.Second())
		})
		It("Should have a trace backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName1, Namespace: mockNs})
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName2, Namespace: mockNs})
		})
		It("Should verify traces from telemetrygen are delivered", func() {
			for _, telemetryExportURL := range telemetryExportURLs {
				verifiers.TracesFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, telemetrygenNs)
			}
		})
	})

	Context("When reaching the pipeline limit", Ordered, func() {
		const maxNumberOfTracePipelines = 3
		var (
			pipelines            = kitkyma.NewPipelineList()
			pipelineCreatedFirst *telemetryv1alpha1.TracePipeline
			pipelineCreatedLater *telemetryv1alpha1.TracePipeline
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			for i := 0; i < maxNumberOfTracePipelines; i++ {
				pipeline := kitk8s.NewTracePipelineV1Alpha1(fmt.Sprintf("pipeline-%d", i))
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
				verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipeline)
				verifiers.TraceCollectorConfigShouldContainPipeline(ctx, k8sClient, pipeline)
			}
		})

		It("Should set ConfigurationGenerated condition to false and Pending condition to true", func() {
			By("Creating an additional pipeline", func() {
				pipeline := kitk8s.NewTracePipelineV1Alpha1("exceeding-pipeline")
				pipelineCreatedLater = pipeline.K8sObject()
				pipelines.Append(pipeline.Name())

				Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline.K8sObject())).Should(Succeed())

				verifiers.TraceCollectorConfigShouldNotContainPipeline(ctx, k8sClient, pipeline.Name())

				var fetched telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: pipeline.Name()}
				Expect(k8sClient.Get(ctx, key, &fetched)).To(Succeed())

				configurationGeneratedCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeConfigurationGenerated)
				Expect(configurationGeneratedCond).NotTo(BeNil())
				Expect(configurationGeneratedCond.Status).Should(Equal(metav1.ConditionFalse))
				Expect(configurationGeneratedCond.Reason).Should(Equal(conditions.ReasonMaxPipelinesExceeded))

				pendingCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypePending)
				Expect(pendingCond).NotTo(BeNil())
				Expect(pendingCond.Status).Should(Equal(metav1.ConditionTrue))
				Expect(pendingCond.Reason).Should(Equal(conditions.ReasonMaxPipelinesExceeded))

				runningCond := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeRunning)
				Expect(runningCond).To(BeNil())
			})
		})

		It("Should have only running pipelines", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineCreatedFirst)).Should(Succeed())

				for _, pipeline := range pipelines.All()[1:] {
					verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, pipeline)
				}
			})
		})
	})

	Context("When a broken tracepipeline exists", Ordered, func() {
		const (
			mockBackendName = "traces-receiver"
			mockNs          = "traces-broken-pipeline"
			telemetrygenNs  = "broken-trace-pipeline-test"
		)
		var (
			healthyPipelineName string
			brokenPipelineName  string
			telemetryExportURL  string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject(),
				kitk8s.NewNamespace(telemetrygenNs).K8sObject(),
			)

			mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
			objs = append(objs, mockBackend.K8sObjects()...)
			telemetryExportURL = mockBackend.TelemetryExportURL(proxyClient)

			healthyPipeline := kitk8s.NewTracePipelineV1Alpha1("healthy").WithOutputEndpointFromSecret(mockBackend.HostSecretRefV1Alpha1())
			healthyPipelineName = healthyPipeline.Name()
			objs = append(objs, healthyPipeline.K8sObject())

			unreachableHostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("trace-host", "http://unreachable:4317"))
			brokenPipeline := kitk8s.NewTracePipelineV1Alpha1("broken").WithOutputEndpointFromSecret(unreachableHostSecret.SecretKeyRefV1Alpha1("trace-host"))
			brokenPipelineName = brokenPipeline.Name()
			objs = append(objs, brokenPipeline.K8sObject(), unreachableHostSecret.K8sObject())

			objs = append(objs,
				telemetrygen.New(telemetrygenNs, telemetrygen.SignalTypeTraces).K8sObject(),
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
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, healthyPipelineName)
			verifiers.TracePipelineShouldBeHealthy(ctx, k8sClient, brokenPipelineName)
		})

		It("Should have a running trace gateway deployment", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, kitkyma.TraceGatewayName)
		})

		It("Should have a trace backend running", func() {
			verifiers.DeploymentShouldBeReady(ctx, k8sClient, types.NamespacedName{Name: mockBackendName, Namespace: mockNs})
		})

		It("Should verify traces from telemetrygen are delivered", func() {
			verifiers.TracesFromNamespaceShouldBeDelivered(proxyClient, telemetryExportURL, telemetrygenNs)
		})
	})
})
