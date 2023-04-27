//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kitmetric "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/matchers"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/metrics"
)

var (
	metricGatewayBaseName = "telemetry-metric-gateway"
)

var _ = Describe("Metrics", func() {
	Context("When a metricpipeline exists", Ordered, func() {
		var (
			portRegistry = testkit.NewPortRegistry().
					AddServicePort("http-otlp", 4318).
					AddPortMapping("grpc-otlp", 4317, 30017, 4317).
					AddPortMapping("http-metrics", 8888, 30088, 8888).
					AddPortMapping("http-web", 80, 30090, 9090)

			otlpPushURL                 = fmt.Sprintf("grpc://localhost:%d", portRegistry.HostPort("grpc-otlp"))
			metricsURL                  = fmt.Sprintf("http://localhost:%d/metrics", portRegistry.HostPort("http-metrics"))
			mockBackendMetricsExportURL = fmt.Sprintf("http://localhost:%d/%s", portRegistry.HostPort("http-web"), telemetryDataFilename)
		)

		BeforeAll(func() {
			k8sObjects := makeMetricsTestK8sObjects(portRegistry)

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
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

		It("Should be able to get metric gateway metrics endpoint", func() {
			Eventually(func(g Gomega) {
				resp, err := http.Get(metricsURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should verify end-to-end metric delivery", func() {
			builder := kitmetrics.NewBuilder()
			var gauges []pmetric.Metric
			for i := 0; i < 50; i++ {
				gauge := kitmetrics.NewGauge()
				gauges = append(gauges, gauge)
				builder.WithMetric(gauge)
			}
			sendMetrics(context.Background(), builder.Build(), otlpPushURL)

			Eventually(func(g Gomega) {
				resp, err := http.Get(mockBackendMetricsExportURL)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					HaveMetrics(gauges...))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

// makeMetricsTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeMetricsTestK8sObjects(portRegistry testkit.PortRegistry) []client.Object {
	var (
		grpcOTLPPort        = portRegistry.ServicePort("grpc-otlp")
		grpcOTLPNodePort    = portRegistry.NodePort("grpc-otlp")
		httpMetricsPort     = portRegistry.ServicePort("http-metrics")
		httpMetricsNodePort = portRegistry.NodePort("http-metrics")
		httpOTLPPort        = portRegistry.ServicePort("http-otlp")
		httpWebPort         = portRegistry.ServicePort("http-web")
		httpWebNodePort     = portRegistry.NodePort("http-web")
	)

	// Mocks namespace objects.
	mocksNamespace := kitk8s.NewNamespace("metric-mocks")
	mockBackend := mocks.NewBackend("metric-receiver", mocksNamespace.Name(), "metrics", "/metrics/"+telemetryDataFilename)
	mockBackendConfigMap := mockBackend.ConfigMap("metric-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPortMapping("http-web", httpWebPort, httpWebNodePort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline := kitmetric.NewPipeline("test", hostSecret.SecretKeyRef("metric-host"))

	// Kyma-system namespace objects.
	metricGatewayExternalService := kitk8s.NewService("telemetry-otlp-metrics-external", kymaSystemNamespaceName).
		WithPortMapping("grpc-otlp", grpcOTLPPort, grpcOTLPNodePort).
		WithPortMapping("http-metrics", httpMetricsPort, httpMetricsNodePort)

	return []client.Object{
		mocksNamespace.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		hostSecret.K8sObject(),
		metricPipeline.K8sObject(),
		metricGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", metricGatewayBaseName)),
	}
}

func sendMetrics(ctx context.Context, metrics pmetric.Metrics, otlpPushURL string) {
	Eventually(func(g Gomega) {
		sender, err := kitmetrics.NewDataSender(otlpPushURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(sender.Start()).Should(Succeed())
		g.Expect(sender.ConsumeMetrics(ctx, metrics)).Should(Succeed())
		sender.Flush()
	}, timeout, interval).Should(Succeed())
}
