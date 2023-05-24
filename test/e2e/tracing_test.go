//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s/verifiers"
	"net/http"
	"time"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/matchers"
	kittraces "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	traceCollectorBaseName             = "telemetry-trace-collector"
	maxNumberOfTracePipelines          = 3
	tracePipelineReconciliationTimeout = 10 * time.Second
)

var _ = Describe("Tracing", func() {
	var (
		portRegistry = testkit.NewPortRegistry().
				AddServicePort("http-otlp", 4318).
				AddPortMapping("grpc-otlp", 4317, 30017, 4317).
				AddPortMapping("http-metrics", 8888, 30088, 8888).
				AddPortMapping("http-web", 80, 30090, 9090)

		otlpPushURL               = fmt.Sprintf("grpc://localhost:%d", portRegistry.HostPort("grpc-otlp"))
		metricsURL                = fmt.Sprintf("http://localhost:%d/metrics", portRegistry.HostPort("http-metrics"))
		mockBackendTraceExportURL = fmt.Sprintf("http://localhost:%d/%s", portRegistry.HostPort("http-web"), telemetryDataFilename)
	)

	Context("When a tracepipeline exists", Ordered, func() {
		mockNs := "trace-mocks-single-pipeline"
		mockDeploymentName := "trace-receiver"

		BeforeAll(func() {
			k8sObjects := makeTracingTestK8sObjects(portRegistry, mockNs, mockDeploymentName)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) bool {
				//var deployment appsv1.Deployment
				key := types.NamespacedName{Name: traceCollectorBaseName, Namespace: kymaSystemNamespaceName}
				res, err := verifiers.IsDeploymentReady(k8sClient, ctx, key)
				g.Expect(err).To(BeNil())
				return res
			}, timeout, interval).Should(BeTrue())
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) bool {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				res, err := verifiers.IsDeploymentReady(k8sClient, ctx, key)
				g.Expect(err).To(BeNil())
				return res

			}, timeout, interval).Should(BeTrue())
		})

		It("Should be able to get trace collector metrics endpoint", func() {
			Eventually(func(g Gomega) {
				resp, err := http.Get(metricsURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running pipeline", func() {
			tracePipelineShouldBeRunning("test")
		})

		It("Should verify end-to-end trace delivery", func() {
			traceID := kittraces.NewTraceID()
			var spanIDs []pcommon.SpanID
			for i := 0; i < 100; i++ {
				spanIDs = append(spanIDs, kittraces.NewSpanID())
			}
			attrs := pcommon.NewMap()
			attrs.PutStr("attrA", "chocolate")
			attrs.PutStr("attrB", "raspberry")
			attrs.PutStr("attrC", "vanilla")

			traces := kittraces.MakeTraces(traceID, spanIDs, attrs)

			sendTraces(context.Background(), traces, otlpPushURL)

			Eventually(func(g Gomega) {
				resp, err := http.Get(mockBackendTraceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfSpansWithIDs(spanIDs),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When reaching the pipeline limit", Ordered, func() {
		allPipelines := make(map[string][]client.Object, 0)

		BeforeAll(func() {
			for i := 0; i < maxNumberOfTracePipelines; i++ {
				pipelineName := fmt.Sprintf("pipeline-%d", i)
				pipelineObjects := makeBrokenTracePipeline(pipelineName)
				allPipelines[pipelineName] = pipelineObjects

				Expect(kitk8s.CreateObjects(ctx, k8sClient, pipelineObjects...)).Should(Succeed())
			}

			DeferCleanup(func() {
				for _, pipeline := range allPipelines {
					Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipeline...)).Should(Succeed())
				}
			})
		})

		It("Should have only running pipelines", func() {
			for pipelineName := range allPipelines {
				tracePipelineShouldBeRunning(pipelineName)
			}
		})

		It("Should have a pending pipeline", func() {
			By("Creating an additional pipeline", func() {
				newPipelineName := "new-pipeline"
				newPipeline := makeBrokenTracePipeline(newPipelineName)
				allPipelines[newPipelineName] = newPipeline

				Expect(kitk8s.CreateObjects(ctx, k8sClient, newPipeline...)).Should(Succeed())
				tracePipelineShouldStayPending(newPipelineName)
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				deletedPipeline := allPipelines["pipeline-0"]
				delete(allPipelines, "pipeline-0")

				Expect(kitk8s.DeleteObjects(ctx, k8sClient, deletedPipeline...)).Should(Succeed())

				for pipelineName := range allPipelines {
					tracePipelineShouldBeRunning(pipelineName)
				}
			})
		})
	})

	Context("When a broken tracepipeline exists", Ordered, func() {
		mockNs := "trace-mocks-broken-pipeline"
		mockDeploymentName := "trace-receiver"

		BeforeAll(func() {
			k8sObjects := makeTracingTestK8sObjects(portRegistry, mockNs, mockDeploymentName)
			secondPipeline := makeBrokenTracePipeline("pipeline-2")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			tracePipelineShouldBeRunning("test")
			tracePipelineShouldBeRunning("pipeline-2")
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) bool {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				res, err := verifiers.IsDeploymentReady(k8sClient, ctx, key)
				g.Expect(err).To(BeNil())
				return res

			}, timeout, interval).Should(BeTrue())
		})

		It("Should verify end-to-end trace delivery for the remaining pipeline", func() {
			traceID := kittraces.NewTraceID()
			var spanIDs []pcommon.SpanID
			for i := 0; i < 100; i++ {
				spanIDs = append(spanIDs, kittraces.NewSpanID())
			}
			attrs := pcommon.NewMap()
			attrs.PutStr("attrA", "chocolate")
			attrs.PutStr("attrB", "raspberry")
			attrs.PutStr("attrC", "vanilla")

			traces := kittraces.MakeTraces(traceID, spanIDs, attrs)

			sendTraces(ctx, traces, otlpPushURL)

			Eventually(func(g Gomega) {
				resp, err := http.Get(mockBackendTraceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfSpansWithIDs(spanIDs),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When multiple tracepipelines exist", Ordered, func() {
		mockNs := "trace-mocks-multi-pipeline"
		mockDeploymentName := "trace-receiver"

		BeforeAll(func() {
			k8sObjects := makeMultiPipelineTracingTestK8sObjects(portRegistry, mockNs, mockDeploymentName)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			tracePipelineShouldBeRunning("pipeline-1")
			tracePipelineShouldBeRunning("pipeline-2")
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) bool {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				res, err := verifiers.IsDeploymentReady(k8sClient, ctx, key)
				g.Expect(err).To(BeNil())
				return res

			}, timeout, interval).Should(BeTrue())
		})

		It("Should verify end-to-end trace delivery", func() {
			traceID := kittraces.NewTraceID()
			var spanIDs []pcommon.SpanID
			for i := 0; i < 100; i++ {
				spanIDs = append(spanIDs, kittraces.NewSpanID())
			}

			attrs := pcommon.NewMap()
			traces := kittraces.MakeTraces(traceID, spanIDs, attrs)

			sendTraces(context.Background(), traces, otlpPushURL)

			// Spans should arrive in the backend twice
			Eventually(func(g Gomega) {
				resp, err := http.Get(mockBackendTraceExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfNumberOfSpans(2 * len(spanIDs)))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func tracePipelineShouldBeRunning(pipelineName string) {
	Eventually(func(g Gomega) bool {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
	}, timeout, interval).Should(BeTrue())
}

func tracePipelineShouldStayPending(pipelineName string) {
	Consistently(func(g Gomega) {
		var pipeline telemetryv1alpha1.TracePipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)).To(BeFalse())
	}, tracePipelineReconciliationTimeout, interval).Should(Succeed())
}

// makeTracingTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeTracingTestK8sObjects(portRegistry testkit.PortRegistry, namespace string, mockDeploymentName string) []client.Object {
	var (
		grpcOTLPPort        = portRegistry.ServicePort("grpc-otlp")
		grpcOTLPNodePort    = portRegistry.NodePort("grpc-otlp")
		httpMetricsPort     = portRegistry.ServicePort("http-metrics")
		httpMetricsNodePort = portRegistry.NodePort("http-metrics")
		httpOTLPPort        = portRegistry.ServicePort("http-otlp")
		httpWebPort         = portRegistry.ServicePort("http-web")
		httpWebNodePort     = portRegistry.NodePort("http-web")
	)

	//// Mocks namespace objects.
	mocksNamespace := kitk8s.NewNamespace(namespace)
	mockBackend := mocks.NewBackend(mockDeploymentName, mocksNamespace.Name(), "/traces/"+telemetryDataFilename, mocks.SignalTypeTraces)
	mockBackendConfigMap := mockBackend.ConfigMap("trace-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPortMapping("http-web", httpWebPort, httpWebNodePort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("trace-host", otlpEndpointURL))
	tracePipeline := kittrace.NewPipeline("test", hostSecret.SecretKeyRef("trace-host"))

	// Kyma-system namespace objects.
	traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kymaSystemNamespaceName).
		WithPortMapping("grpc-otlp", grpcOTLPPort, grpcOTLPNodePort).
		WithPortMapping("http-metrics", httpMetricsPort, httpMetricsNodePort)

	return []client.Object{
		mocksNamespace.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		hostSecret.K8sObject(),
		tracePipeline.K8sObject(),
		traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", traceCollectorBaseName)),
	}
}

// makeMultiPipelineTracingTestK8sObjects returns the list of mandatory E2E test suite k8s objects including two tracepipelines.
func makeMultiPipelineTracingTestK8sObjects(portRegistry testkit.PortRegistry, namespace string, mockDeploymentName string) []client.Object {
	var (
		grpcOTLPPort        = portRegistry.ServicePort("grpc-otlp")
		grpcOTLPNodePort    = portRegistry.NodePort("grpc-otlp")
		httpMetricsPort     = portRegistry.ServicePort("http-metrics")
		httpMetricsNodePort = portRegistry.NodePort("http-metrics")
		httpOTLPPort        = portRegistry.ServicePort("http-otlp")
		httpWebPort         = portRegistry.ServicePort("http-web")
		httpWebNodePort     = portRegistry.NodePort("http-web")
	)

	//// Mocks namespace objects.
	mocksNamespace := kitk8s.NewNamespace(namespace)
	mockBackend := mocks.NewBackend(mockDeploymentName, mocksNamespace.Name(), "/traces/"+telemetryDataFilename, mocks.SignalTypeTraces)
	mockBackendConfigMap := mockBackend.ConfigMap("trace-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPortMapping("http-web", httpWebPort, httpWebNodePort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret1 := kitk8s.NewOpaqueSecret("trace-rcv-hostname-1", defaultNamespaceName, kitk8s.WithStringData("trace-host", otlpEndpointURL))
	tracePipeline1 := kittrace.NewPipeline("pipeline-1", hostSecret1.SecretKeyRef("trace-host"))

	hostSecret2 := kitk8s.NewOpaqueSecret("trace-rcv-hostname-2", defaultNamespaceName, kitk8s.WithStringData("trace-host", otlpEndpointURL))
	tracePipeline2 := kittrace.NewPipeline("pipeline-2", hostSecret2.SecretKeyRef("trace-host"))

	// Kyma-system namespace objects.
	traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kymaSystemNamespaceName).
		WithPortMapping("grpc-otlp", grpcOTLPPort, grpcOTLPNodePort).
		WithPortMapping("http-metrics", httpMetricsPort, httpMetricsNodePort)

	return []client.Object{
		mocksNamespace.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		hostSecret1.K8sObject(),
		tracePipeline1.K8sObject(),
		hostSecret2.K8sObject(),
		tracePipeline2.K8sObject(),
		traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", traceCollectorBaseName)),
	}
}

func makeBrokenTracePipeline(name string) []client.Object {
	hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("trace-host", "http://unreachable:4317"))
	tracePipeline := kittrace.NewPipeline(name, hostSecret.SecretKeyRef("trace-host"))

	return []client.Object{
		hostSecret.K8sObject(),
		tracePipeline.K8sObject(),
	}
}

func sendTraces(ctx context.Context, traces ptrace.Traces, otlpPushURL string) {
	Eventually(func(g Gomega) {
		sender, err := kittraces.NewDataSender(otlpPushURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sender.Start()).Should(Succeed())
		g.Expect(sender.ConsumeTraces(ctx, traces)).Should(Succeed())
		sender.Flush()
	}, timeout, interval).Should(Succeed())
}
