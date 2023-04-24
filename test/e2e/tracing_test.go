//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/kyma-project/telemetry-manager/internal/otelmatchers"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/trace"
	kitmocks "github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
	kittraces "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"
)

var (
	traceCollectorServiceName = "telemetry-trace-collector"
)

var _ = Describe("Tracing", func() {
	Context("When a tracepipeline exists", Ordered, func() {
		var (
			portRegistry = testkit.NewPortRegistry().
					AddServicePort("http-otlp", 4318).
					AddPortMapping("grpc-otlp", 4317, 30017, 4317).
					AddPortMapping("http-metrics", 8888, 30088, 8888).
					AddPortMapping("http-web", 80, 30090, 9090)

			otlpPushURL               = fmt.Sprintf("grpc://localhost:%d", portRegistry.HostPort("grpc-otlp"))
			metricsURL                = fmt.Sprintf("http://localhost:%d/metrics", portRegistry.HostPort("http-metrics"))
			mockBackendTraceExportURL = fmt.Sprintf("http://localhost:%d/otlp-data.jsonl", portRegistry.HostPort("http-web"))
		)

		BeforeAll(func() {
			k8sObjects := makeTracingTestK8sObjects(portRegistry)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: traceCollectorServiceName, Namespace: kymaSystemNamespaceName}
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
})

// makeTracingTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeTracingTestK8sObjects(portRegistry testkit.PortRegistry) []client.Object {
	const (
		pipelineName                  = "test"
		hostSecretName                = "trace-rcv-hostname" //nolint:gosec // Is not a hardcoded credential.
		hostSecretNamespace           = "default"
		hostSecretKey                 = "trace-host"
		mockBackendName               = "trace-receiver"
		mocksNamespaceName            = "trace-mocks"
		mockBackendConfigMapName      = "trace-receiver-config"
		otlpExternalTracesServiceName = "telemetry-otlp-traces-external"
	)

	var (
		grpcOTLPPort        = portRegistry.ServicePort("grpc-otlp")
		grpcOTLPNodePort    = portRegistry.NodePort("grpc-otlp")
		httpMetricsPort     = portRegistry.ServicePort("http-metrics")
		httpMetricsNodePort = portRegistry.NodePort("http-metrics")
		httpOTLPPort        = portRegistry.ServicePort("http-otlp")
		httpWebPort         = portRegistry.ServicePort("http-web")
		httpWebNodePort     = portRegistry.NodePort("http-web")
	)

	mocksNamespace := kitk8s.NewNamespace(mocksNamespaceName)
	mockBackendConfigMap := kitmocks.NewBackendConfigMap(mockBackendConfigMapName, mocksNamespaceName)
	mockBackendDeployment := kitmocks.NewBackendDeployment(mockBackendName, mocksNamespaceName, mockBackendConfigMapName)
	externalMockBackendService := kitmocks.NewExternalBackendService(mockBackendName, mocksNamespaceName).
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPortMapping("http-web", httpWebPort, httpWebNodePort)

	mockBackendOTLPURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", mockBackendName, mocksNamespaceName, grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret(hostSecretName, hostSecretNamespace, kitk8s.WithStringData(hostSecretKey, mockBackendOTLPURL))
	tracePipeline := kittrace.NewPipeline(pipelineName, hostSecret.SecretKeyRef(hostSecretKey))
	externalTraceService := kitk8s.NewService(otlpExternalTracesServiceName, kymaSystemNamespaceName).
		WithPortMapping("grpc-otlp", grpcOTLPPort, grpcOTLPNodePort).
		WithPortMapping("http-metrics", httpMetricsPort, httpMetricsNodePort)

	return []client.Object{
		mocksNamespace.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackendName)),
		externalMockBackendService.K8sObject(kitk8s.WithLabel("app", mockBackendName)),
		hostSecret.K8sObject(),
		tracePipeline.K8sObject(),
		externalTraceService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", traceCollectorServiceName)),
	}
}

func sendTraces(ctx context.Context, traces ptrace.Traces, otlpPushURL string) {
	Eventually(func(g Gomega) {
		sender, err := kittraces.NewDataSender(otlpPushURL)
		Expect(err).NotTo(HaveOccurred())
		Expect(sender.Start()).Should(Succeed())
		Expect(sender.ConsumeTraces(ctx, traces)).Should(Succeed())
		sender.Flush()
	}, timeout, interval).Should(Succeed())
}
