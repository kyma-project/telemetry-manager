//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s/verifiers"
	kittrace "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/trace"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
	kittraces "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/matchers"
)

var (
	traceCollectorBaseName             = "telemetry-trace-collector"
	maxNumberOfTracePipelines          = 3
	tracePipelineReconciliationTimeout = 10 * time.Second
)

var _ = Describe("Tracing", func() {
	Context("When a tracepipeline exists", Ordered, func() {
		var (
			urls               *mocks.URLProvider
			mockNs             = "trace-mocks-single-pipeline"
			mockDeploymentName = "trace-receiver"
		)

		BeforeAll(func() {
			k8sObjects, tracesURLProvider := makeTracingTestK8sObjects(mockNs, mockDeploymentName)
			urls = tracesURLProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: traceCollectorBaseName, Namespace: kymaSystemNamespaceName}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should be able to get trace collector metrics endpoint", func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.Metrics())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running pipeline", func() {
			tracePipelineShouldBeRunning("pipeline")
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

			Expect(sendTraces(context.Background(), traces, urls.OTLPPush())).To(Succeed())

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfSpansWithIDs(spanIDs),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a working network policy", func() {
			var networkPolicy networkingv1.NetworkPolicy
			key := types.NamespacedName{Name: traceCollectorBaseName + "-pprof-deny-ingress", Namespace: kymaSystemNamespaceName}
			Expect(k8sClient.Get(ctx, key, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kymaSystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": traceCollectorBaseName})).To(Succeed())
				g.Expect(podList.Items).To(HaveLen(1))

				traceCollectorPodName := podList.Items[0].Name
				pprofPort := 1777
				pprofEndpoint := proxyClient.ProxyURLForPod(kymaSystemNamespaceName, traceCollectorPodName, "debug/pprof/", pprofPort)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
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
		var (
			urls               *mocks.URLProvider
			mockNs             = "trace-mocks-broken-pipeline"
			mockDeploymentName = "trace-receiver"
		)

		BeforeAll(func() {
			k8sObjects, tracesURLProvider := makeTracingTestK8sObjects(mockNs, mockDeploymentName)
			urls = tracesURLProvider
			secondPipeline := makeBrokenTracePipeline("pipeline-1")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			tracePipelineShouldBeRunning("pipeline")
			tracePipelineShouldBeRunning("pipeline-1")
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
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

			Expect(sendTraces(ctx, traces, urls.OTLPPush())).To(Succeed())

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
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
		var (
			urls                        *mocks.URLProvider
			mockNs                      = "trace-mocks-multi-pipeline"
			primaryMockDeploymentName   = "trace-receiver"
			auxiliaryMockDeploymentName = "trace-receiver-1"
		)

		BeforeAll(func() {
			k8sObjects, tracesURLProvider := makeTracingTestK8sObjects(mockNs, primaryMockDeploymentName, auxiliaryMockDeploymentName)
			urls = tracesURLProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			tracePipelineShouldBeRunning("pipeline")
			tracePipelineShouldBeRunning("pipeline-1")
		})

		It("Should have a trace backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: primaryMockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify end-to-end trace delivery", func() {
			traceID := kittraces.NewTraceID()
			var spanIDs []pcommon.SpanID
			for i := 0; i < 100; i++ {
				spanIDs = append(spanIDs, kittraces.NewSpanID())
			}

			attrs := pcommon.NewMap()
			traces := kittraces.MakeTraces(traceID, spanIDs, attrs)

			Expect(sendTraces(context.Background(), traces, urls.OTLPPush())).To(Succeed())

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfNumberOfSpans(len(spanIDs)),
					ConsistOfSpansWithIDs(spanIDs),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExportAt(1))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfNumberOfSpans(len(spanIDs)),
					ConsistOfSpansWithIDs(spanIDs),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
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
func makeTracingTestK8sObjects(namespace string, mockDeploymentNames ...string) ([]client.Object, *mocks.URLProvider) {
	var (
		objs []client.Object
		urls = mocks.NewURLProvider()

		grpcOTLPPort    = 4317
		httpMetricsPort = 8888
		httpOTLPPort    = 4318
		httpWebPort     = 80
	)

	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, kitk8s.NewNamespace(namespace).K8sObject())

	for i, mockDeploymentName := range mockDeploymentNames {
		//// Mocks namespace objects.
		mockBackend := mocks.NewBackend(suffixize(mockDeploymentName, i), mocksNamespace.Name(), "/traces/"+telemetryDataFilename, mocks.SignalTypeTraces)
		mockBackendConfigMap := mockBackend.ConfigMap(suffixize("trace-receiver-config", i))
		mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
		mockBackendExternalService := mockBackend.ExternalService().
			WithPort("grpc-otlp", grpcOTLPPort).
			WithPort("http-otlp", httpOTLPPort).
			WithPort("http-web", httpWebPort)

		// Default namespace objects.
		otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
		hostSecret := kitk8s.NewOpaqueSecret(suffixize("trace-rcv-hostname", i), defaultNamespaceName, kitk8s.WithStringData("trace-host", otlpEndpointURL))
		tracePipeline := kittrace.NewPipeline(suffixize("pipeline", i), hostSecret.SecretKeyRef("trace-host"))

		objs = append(objs, []client.Object{
			mockBackendConfigMap.K8sObject(),
			mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			hostSecret.K8sObject(),
			tracePipeline.K8sObject(),
		}...)

		urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), i)
	}

	urls.SetOTLPPush(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-traces", "v1/traces/", httpOTLPPort))

	// Kyma-system namespace objects.
	traceGatewayExternalService := kitk8s.NewService("telemetry-otlp-traces-external", kymaSystemNamespaceName).
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-metrics", httpMetricsPort)
	urls.SetMetrics(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-traces-external", "metrics", httpMetricsPort))

	objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", traceCollectorBaseName)))

	return objs, urls
}

func makeBrokenTracePipeline(name string) []client.Object {
	hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("trace-host", "http://unreachable:4317"))
	tracePipeline := kittrace.NewPipeline(name, hostSecret.SecretKeyRef("trace-host"))

	return []client.Object{
		hostSecret.K8sObject(),
		tracePipeline.K8sObject(),
	}
}

func sendTraces(ctx context.Context, traces ptrace.Traces, otlpPushURL string) error {
	sender, err := kittraces.NewHTTPSender(ctx, otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}

	return sender.Export(ctx, traces)
}
