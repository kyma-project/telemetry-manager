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
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otel/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces Multi-Pipeline", Label("traces"), func() {
	Context("When multiple tracepipelines exist", Ordered, func() {
		const (
			mockNs           = "traces-multi-pipeline"
			mockBackendName1 = "trace-receiver-1"
			mockBackendName2 = "trace-receiver-2"
		)
		var (
			pipelines = kitkyma.NewPipelineList()
			urls      = urlprovider.New()
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			for _, backendName := range []string{mockBackendName1, mockBackendName2} {
				mockBackend := backend.New(backendName, mockNs, backend.SignalTypeTraces)
				objs = append(objs, mockBackend.K8sObjects()...)
				urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

				pipeline := kitk8s.NewTracePipeline(fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline")).WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
				pipelines.Append(pipeline.Name())
				objs = append(objs, pipeline.K8sObject())
			}

			urls.SetOTLPPush(proxyClient.ProxyURLForService(
				kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
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
		It("Should verify end-to-end trace delivery", func() {
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName1), traceID, spanIDs, attrs)
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName2), traceID, spanIDs, attrs)
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
				pipeline := kitk8s.NewTracePipeline(fmt.Sprintf("pipeline-%d", i))
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
				pipeline := kitk8s.NewTracePipeline("exceeding-pipeline")
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
		)
		var (
			urls                = urlprovider.New()
			healthyPipelineName string
			brokenPipelineName  string
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			mockBackend := backend.New(mockBackendName, mockNs, backend.SignalTypeTraces)
			objs = append(objs, mockBackend.K8sObjects()...)
			urls.SetMockBackendExport(mockBackend.Name(), mockBackend.TelemetryExportURL(proxyClient))

			healthyPipeline := kitk8s.NewTracePipeline("healthy").WithOutputEndpointFromSecret(mockBackend.HostSecretRef())
			healthyPipelineName = healthyPipeline.Name()
			objs = append(objs, healthyPipeline.K8sObject())

			unreachableHostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-broken", kitkyma.DefaultNamespaceName,
				kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
			brokenPipeline := kitk8s.NewTracePipeline("broken").WithOutputEndpointFromSecret(unreachableHostSecret.SecretKeyRef("metric-host"))
			brokenPipelineName = brokenPipeline.Name()
			objs = append(objs, brokenPipeline.K8sObject(), unreachableHostSecret.K8sObject())

			urls.SetOTLPPush(proxyClient.ProxyURLForService(
				kitkyma.SystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
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

		It("Should verify end-to-end trace delivery for the remaining pipeline", func() {
			traceID, spanIDs, attrs := kittraces.MakeAndSendTraces(proxyClient, urls.OTLPPush())
			verifiers.TracesShouldBeDelivered(proxyClient, urls.MockBackendExport(mockBackendName), traceID, spanIDs, attrs)
		})
	})
})
