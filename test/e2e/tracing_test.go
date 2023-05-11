//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"time"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/matchers"
	kittraces "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"
)

var (
	traceCollectorBaseName = "telemetry-trace-collector"
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
		BeforeAll(func() {
			k8sObjects := makeTracingTestK8sObjects(portRegistry, "trace-mocks-single-pipeline")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: traceCollectorBaseName, Namespace: kymaSystemNamespaceName}
				g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())

				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
					Namespace:     kymaSystemNamespaceName,
				}
				var pods corev1.PodList
				Expect(k8sClient.List(ctx, &pods, &listOptions)).To(Succeed())
				for _, pod := range pods.Items {
					for _, containerStatus := range pod.Status.ContainerStatuses {
						if containerStatus.State.Running == nil {
							return false
						}
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should be able to get trace collector metrics endpoint", func() {
			Eventually(func(g Gomega) {
				resp, err := http.Get(metricsURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
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

	Context("When reaching the pipeline limit", func() {
		It("Should have 3 running pipelines", func() {
			pipeline1Objects := makeBrokenTracePipeline("pipeline-1")
			pipeline2Objects := makeBrokenTracePipeline("pipeline-2")
			pipeline3Objects := makeBrokenTracePipeline("pipeline-3")
			pipeline4Objects := makeBrokenTracePipeline("pipeline-4")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipeline2Objects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipeline3Objects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipeline4Objects...)).Should(Succeed())
			})

			Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline1Objects...)).Should(Succeed())
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-1"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())

			Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline2Objects...)).Should(Succeed())
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-2"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())

			Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline3Objects...)).Should(Succeed())
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-3"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())

			Expect(kitk8s.CreateObjects(ctx, k8sClient, pipeline4Objects...)).Should(Succeed())
			time.Sleep(10 * time.Second) // wait for reconciliation
			var pipeline telemetryv1alpha1.TracePipeline
			key := types.NamespacedName{Name: "pipeline-4"}
			Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
			// We allow only 3 piplines, pipeline-4 should not be running
			Expect(!pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning))

			Expect(kitk8s.DeleteObjects(ctx, k8sClient, pipeline1Objects...)).Should(Succeed())
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-4"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				// pipeline-4 should become running after pipeline-1 is deleted
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When a broken tracepipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeTracingTestK8sObjects(portRegistry, "trace-mocks-broken-pipeline")
			secondPipeline := makeBrokenTracePipeline("pipeline-2")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "test"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())

			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-2"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
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
		BeforeAll(func() {
			k8sObjects := makeMultiPipelineTracingTestK8sObjects(portRegistry, "trace-mocks-multi-pipeline")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-1"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
			}, timeout, interval).Should(BeTrue())

			Eventually(func(g Gomega) bool {
				var pipeline telemetryv1alpha1.TracePipeline
				key := types.NamespacedName{Name: "pipeline-2"}
				g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
				return pipeline.Status.HasCondition(telemetryv1alpha1.TracePipelineRunning)
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

// makeTracingTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeTracingTestK8sObjects(portRegistry testkit.PortRegistry, namespace string) []client.Object {
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
	mockBackend := mocks.NewBackend("trace-receiver", mocksNamespace.Name(), "/traces/"+telemetryDataFilename, mocks.SignalTypeTraces)
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
func makeMultiPipelineTracingTestK8sObjects(portRegistry testkit.PortRegistry, namespace string) []client.Object {
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
	mockBackend := mocks.NewBackend("trace-receiver", mocksNamespace.Name(), "/traces/"+telemetryDataFilename, mocks.SignalTypeTraces)
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
