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

	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"

	. "github.com/kyma-project/telemetry-manager/internal/otelmatchers"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kittrace "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/trace"
	tracesmocks "github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks/traces"
	kitotlp "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp"
	kittraces "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/traces"
)

const (
	externalTracesBackend     = "trace-receiver"
	secretName                = "trace-rcv-hostname" //nolint:gosec // Is not a hardcoded credential.
	otlpExternalTracesService = "telemetry-otlp-traces-external"
	traceCollectorService     = "telemetry-trace-collector"
	traceHostSecretKey        = "trace-host"
	tracePipeline             = "test"
	traceReceiverConfigName   = "trace-receiver-config"
)

var _ = Describe("Tracing", func() {
	Context("When a tracepipeline exists", Ordered, func() {
		var (
			portRegistry = testkit.NewPortRegistry().
					AddPort("http-otlp", 4318).
					AddPortMapping("grpc-otlp", 4317, 30017).
					AddPortMapping("http-metrics", 8888, 30088).
					AddPortMapping("http-web", 80, 30090)

			traceHostURL      = fmt.Sprintf("http://%s.mocks.svc.cluster.local:%d", externalTracesBackend, portRegistry.Port("grpc-otlp"))
			httpTracesURL     = fmt.Sprintf("http://localhost:%d/metrics", portRegistry.Port("http-metrics"))
			httpTracesMockURL = fmt.Sprintf("http://localhost:9090/spans.json")
		)

		BeforeAll(func() {
			k8sObjects := makeK8sObjects(portRegistry, traceHostURL)

			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
		})

		It("Should have a running trace collector deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: traceCollectorService, Namespace: kymaSystemNamespace}
				g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())

				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
					Namespace:     kymaSystemNamespace,
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
				resp, err := http.Get(httpTracesURL)
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

			sendTraces(context.Background(), traces, "localhost", portRegistry.Port("grpc-otlp"))

			Eventually(func(g Gomega) {
				resp, err := http.Get(httpTracesMockURL)
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

// makeK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeK8sObjects(portRegistry testkit.PortRegistry, traceHostURL string) []client.Object {
	var (
		grpcOtlpPort        = portRegistry.Port("grpc-otlp")
		grpcOtlpNodePort    = portRegistry.NodePort("grpc-otlp")
		httpMetricsPort     = portRegistry.Port("http-metrics")
		httpMetricsNodePort = portRegistry.NodePort("http-metrics")
		httpOtlpPort        = portRegistry.Port("http-otlp")
		httpWebPort         = portRegistry.Port("http-web")
		httpWebNodePort     = portRegistry.NodePort("http-web")
	)

	secret := kitk8s.NewOpaqueSecret(secretName, telemetryNamespace, kitk8s.WithStringData(traceHostSecretKey, traceHostURL))
	tracePipelineResource := kittrace.NewPipeline(tracePipeline, secret.SecretKeyRef(traceHostSecretKey))
	externalTraceService := kitotlp.NewTracesService(otlpExternalTracesService, kymaSystemNamespace).
		WithPortMapping("grpc-otlp", grpcOtlpPort, grpcOtlpNodePort).
		WithPortMapping("http-metrics", httpMetricsPort, httpMetricsNodePort)

	// Tracing Mocks.
	mocksNamespaceResource := kitk8s.NewNamespace(mocksNamespace)
	mockBackendConfigMap := tracesmocks.NewBackendConfigMap(traceReceiverConfigName, mocksNamespace)
	mockBackendDeployment := tracesmocks.NewBackendDeployment(externalTracesBackend, mocksNamespace, traceReceiverConfigName)
	externalMockBackendService := tracesmocks.NewExternalBackendService(externalTracesBackend, mocksNamespace).
		WithPort("grpc-otlp", grpcOtlpPort).
		WithPort("http-otlp", httpOtlpPort).
		WithPortMapping("http-web", httpWebPort, httpWebNodePort)

	return []client.Object{
		secret.K8sObject(),
		tracePipelineResource.K8sObject(),
		externalTraceService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", traceCollectorService)),
		mocksNamespaceResource.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", externalTracesBackend)),
		externalMockBackendService.K8sObject(kitk8s.WithLabel("app", externalTracesBackend)),
	}
}

func sendTraces(ctx context.Context, traces ptrace.Traces, host string, port int32) {
	sender := testbed.NewOTLPTraceDataSender(host, int(port))
	Expect(sender.Start()).Should(Succeed())
	Expect(sender.ConsumeTraces(ctx, traces)).Should(Succeed())
	sender.Flush()
}
