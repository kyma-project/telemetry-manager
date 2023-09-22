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
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"
	"github.com/kyma-project/telemetry-manager/test/testkit/verifiers"
)

var _ = Describe("Traces", Label("tracing"), func() {
	Context("When multiple tracepipelines exist", Ordered, func() {
		const (
			mockNs           = "trace-mocks-multi-pipeline"
			mockBackendName1 = "trace-receiver-1"
			mockBackendName2 = "trace-receiver-2"
		)
		var (
			pipelines = kyma.NewPipelineList()
			urls      = urlprovider.New()
		)

		makeResources := func() []client.Object {
			var objs []client.Object
			objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

			for _, backendName := range []string{mockBackendName1, mockBackendName2} {
				mockBackend := backend.New(backendName, mockNs, backend.SignalTypeTraces)
				objs = append(objs, mockBackend.K8sObjects()...)
				urls.SetMockBackendExport(mockBackend.Name(), proxyClient.ProxyURLForService(
					mockNs, mockBackend.Name(), backend.TelemetryDataFilename, backend.HTTPWebPort),
				)

				pipeline := kittrace.NewPipeline(
					fmt.Sprintf("%s-%s", mockBackend.Name(), "pipeline"),
					mockBackend.HostSecretRefKey(),
				)
				pipelines.Append(pipeline.Name())
				objs = append(objs, pipeline.K8sObject())
			}

			urls.SetOTLPPush(proxyClient.ProxyURLForService(
				kymaSystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", ports.OTLPHTTP),
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
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelines.First())
			verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipelines.Second())
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
			pipelineObjectsToDelete []client.Object
			pipelines               = kyma.NewPipelineList()
		)

		makeResources := func() []client.Object {
			var allObjs []client.Object
			for i := 0; i < maxNumberOfTracePipelines; i++ {
				name := fmt.Sprintf("pipeline-%d", i)
				hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
				brokenTracePipeline := kittrace.NewPipeline(name, hostSecret.SecretKeyRef("metric-host"))
				objs := []client.Object{hostSecret.K8sObject(), brokenTracePipeline.K8sObject()}
				pipelines.Append(name)
				allObjs = append(allObjs, objs...)
				if i == 0 {
					pipelineObjectsToDelete = objs
				}

				Expect(kitk8s.CreateObjects(ctx, k8sClient, objs...)).Should(Succeed())
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
				verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipeline)
				verifiers.TraceCollectorConfigShouldContainPipeline(ctx, k8sClient, pipeline)
			}
		})

		It("Should have a pending pipeline", func() {
			By("Creating an additional pipeline", func() {
				name := "exceeding-pipeline"
				hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
				brokenTracePipeline := kittrace.NewPipeline(name, hostSecret.SecretKeyRef("metric-host"))
				newObjs := []client.Object{hostSecret.K8sObject(), brokenTracePipeline.K8sObject()}
				pipelines.Append(name)

				Expect(kitk8s.CreateObjects(ctx, k8sClient, newObjs...)).Should(Succeed())
				verifiers.TracePipelineShouldStayPending(ctx, k8sClient, name)
				verifiers.TraceCollectorConfigShouldNotContainPipeline(ctx, k8sClient, name)
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipelineObjectsToDelete...)).Should(Succeed())

				for _, pipeline := range pipelines.All()[1:] {
					verifiers.TracePipelineShouldBeRunning(ctx, k8sClient, pipeline)
				}
			})
		})
	})

})
