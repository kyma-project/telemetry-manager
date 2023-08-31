//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/k8s/verifiers"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kittrace "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/trace"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks"
	kittraces "github.com/kyma-project/telemetry-manager/test/testkit/otlp/traces"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	traceGatewayBaseName      = "telemetry-trace-collector"
	maxNumberOfTracePipelines = 3
)

var _ = Describe("Tracing", Label("tracing"), func() {
	Context("When a tracepipeline exists", Ordered, func() {
		var (
			pipelines          *kyma.PipelineList
			urls               *mocks.URLProvider
			mockNs             = "trace-mocks-single-pipeline"
			mockDeploymentName = "trace-receiver"
			traceGatewayName   = types.NamespacedName{Name: traceGatewayBaseName, Namespace: kymaSystemNamespaceName}
		)

		BeforeAll(func() {
			k8sObjects, tracesURLProvider, pipelinesProvider := makeTracingTestK8sObjects(mockNs, mockDeploymentName)
			pipelines = pipelinesProvider
			urls = tracesURLProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace gateway deployment", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, traceGatewayName)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have 2 trace gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				err := k8sClient.Get(ctx, traceGatewayName, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(2)))
		})

		It("Should have a trace backend running", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should be able to get trace gateway metrics endpoint", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.Metrics())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running pipeline", Label(operationalTest), func() {
			tracePipelineShouldBeRunning(pipelines.First())
		})

		It("Should verify end-to-end trace delivery", Label(operationalTest), func() {
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
					ConsistOfSpansWithIDs(spanIDs...),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a working network policy", func() {
			var networkPolicy networkingv1.NetworkPolicy
			key := types.NamespacedName{Name: traceGatewayBaseName + "-pprof-deny-ingress", Namespace: kymaSystemNamespaceName}
			Expect(k8sClient.Get(ctx, key, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kymaSystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": traceGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				traceGatewayPodName := podList.Items[0].Name
				pprofPort := 1777
				pprofEndpoint := proxyClient.ProxyURLForPod(kymaSystemNamespaceName, traceGatewayPodName, "debug/pprof/", pprofPort)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When tracepipeline with missing secret reference exists", Ordered, func() {
		hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("trace-host", "http://localhost:4317"))
		tracePipeline := kittrace.NewPipeline("without-secret", hostSecret.SecretKeyRef("trace-host"))

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, tracePipeline.K8sObject())).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, tracePipeline.K8sObject(), hostSecret.K8sObject())).Should(Succeed())
			})
		})

		It("Should have pending tracepipeline", func() {
			tracePipelineShouldStayPending(tracePipeline.Name())
		})

		It("Should not have trace-collector deployment", func() {
			Consistently(func(g Gomega) {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: "telemetry-trace-collector", Namespace: "kyma-system"}
				g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
			}, reconciliationTimeout, interval).ShouldNot(Succeed())
		})

		It("Should have running tracepipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, hostSecret.K8sObject())).Should(Succeed())
			})

			tracePipelineShouldBeRunning(tracePipeline.Name())
		})
	})

	Context("When reaching the pipeline limit", Ordered, func() {
		pipelinesObjects := make(map[string][]client.Object, 0)
		pipelines := kyma.NewPipelineList()

		BeforeAll(func() {
			for i := 0; i < maxNumberOfTracePipelines; i++ {
				objs, name := makeBrokenTracePipeline(fmt.Sprintf("pipeline-%d", i))
				pipelines.Append(name)
				pipelinesObjects[name] = objs

				Expect(kitk8s.CreateObjects(ctx, k8sClient, objs...)).Should(Succeed())
			}

			DeferCleanup(func() {
				for _, objs := range pipelinesObjects {
					Expect(kitk8s.DeleteObjects(ctx, k8sClient, objs...)).Should(Succeed())
				}
			})
		})

		It("Should have only running pipelines", func() {
			for _, pipeline := range pipelines.All() {
				tracePipelineShouldBeRunning(pipeline)
				tracePipelineShouldBeDeployed(pipeline)
			}
		})

		It("Should have a pending pipeline", func() {
			By("Creating an additional pipeline", func() {
				newObjs, newName := makeBrokenTracePipeline("exceeding-pipeline")
				pipelinesObjects[newName] = newObjs
				pipelines.Append(newName)

				Expect(kitk8s.CreateObjects(ctx, k8sClient, newObjs...)).Should(Succeed())
				tracePipelineShouldStayPending(newName)
				tracePipelineShouldNotBeDeployed(newName)
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				deletedPipeline := pipelinesObjects[pipelines.First()]
				delete(pipelinesObjects, pipelines.First())

				Expect(kitk8s.DeleteObjects(ctx, k8sClient, deletedPipeline...)).Should(Succeed())

				for _, pipeline := range pipelines.All()[1:] {
					tracePipelineShouldBeRunning(pipeline)
				}
			})
		})
	})

	Context("When a broken tracepipeline exists", Ordered, func() {
		var (
			brokenPipelineName string
			pipelines          *kyma.PipelineList
			urls               *mocks.URLProvider
			mockNs             = "trace-mocks-broken-pipeline"
			mockDeploymentName = "trace-receiver"
		)

		BeforeAll(func() {
			k8sObjects, tracesURLProvider, pipelinesProvider := makeTracingTestK8sObjects(mockNs, mockDeploymentName)
			pipelines = pipelinesProvider
			urls = tracesURLProvider
			brokenPipelineObjs, brokenName := makeBrokenTracePipeline("pipeline-1")
			brokenPipelineName = brokenName

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, brokenPipelineObjs...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, brokenPipelineObjs...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			tracePipelineShouldBeRunning(pipelines.First())
			tracePipelineShouldBeRunning(brokenPipelineName)
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
					ConsistOfSpansWithIDs(spanIDs...),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When multiple tracepipelines exist", Ordered, func() {
		var (
			pipelines                   *kyma.PipelineList
			urls                        *mocks.URLProvider
			mockNs                      = "trace-mocks-multi-pipeline"
			primaryMockDeploymentName   = "trace-receiver"
			auxiliaryMockDeploymentName = "trace-receiver-1"
		)

		BeforeAll(func() {
			k8sObjects, tracesURLProvider, pipelinesProvider := makeTracingTestK8sObjects(mockNs, primaryMockDeploymentName, auxiliaryMockDeploymentName)
			pipelines = pipelinesProvider
			urls = tracesURLProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			tracePipelineShouldBeRunning(pipelines.First())
			tracePipelineShouldBeRunning(pipelines.Second())
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
					ConsistOfSpansWithIDs(spanIDs...),
					ConsistOfSpansWithTraceID(traceID),
					ConsistOfSpansWithAttributes(attrs))))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExportAt(1))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ConsistOfNumberOfSpans(len(spanIDs)),
					ConsistOfSpansWithIDs(spanIDs...),
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
	}, reconciliationTimeout, interval).Should(Succeed())
}

func tracePipelineShouldBeDeployed(pipelineName string) {
	Eventually(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		key := types.NamespacedName{Name: "telemetry-trace-collector", Namespace: "kyma-system"}
		g.Expect(k8sClient.Get(ctx, key, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return strings.Contains(configString, pipelineAlias)
	}, timeout, interval).Should(BeTrue())
}

func tracePipelineShouldNotBeDeployed(pipelineName string) {
	Consistently(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		key := types.NamespacedName{Name: "telemetry-trace-collector", Namespace: "kyma-system"}
		g.Expect(k8sClient.Get(ctx, key, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return !strings.Contains(configString, pipelineAlias)
	}, reconciliationTimeout, interval).Should(BeTrue())
}

// makeTracingTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeTracingTestK8sObjects(namespace string, mockDeploymentNames ...string) ([]client.Object, *mocks.URLProvider, *kyma.PipelineList) {
	var (
		objs      []client.Object
		pipelines = kyma.NewPipelineList()
		urls      = mocks.NewURLProvider()

		grpcOTLPPort    = 4317
		httpMetricsPort = 8888
		httpOTLPPort    = 4318
		httpWebPort     = 80
	)

	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, mocksNamespace.K8sObject())

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
		hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("trace-host", otlpEndpointURL)).Persistent(isOperational())
		tracePipeline := kittrace.NewPipeline(fmt.Sprintf("%s-%s", mockDeploymentName, "pipeline"), hostSecret.SecretKeyRef("trace-host")).Persistent(isOperational())
		pipelines.Append(tracePipeline.Name())

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

	objs = append(objs, traceGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", traceGatewayBaseName)))

	return objs, urls, pipelines
}

func makeBrokenTracePipeline(name string) ([]client.Object, string) {
	hostSecret := kitk8s.NewOpaqueSecret("trace-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("trace-host", "http://unreachable:4317"))
	tracePipeline := kittrace.NewPipeline(name, hostSecret.SecretKeyRef("trace-host"))

	return []client.Object{
		hostSecret.K8sObject(),
		tracePipeline.K8sObject(),
	}, tracePipeline.Name()
}

func sendTraces(ctx context.Context, traces ptrace.Traces, otlpPushURL string) error {
	sender, err := kittraces.NewHTTPSender(ctx, otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}

	return sender.Export(ctx, traces)
}
