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
	metricGatewayBaseName               = "telemetry-metric-gateway"
	maxNumberOfMetricPipelines          = 3
	metricPipelineReconciliationTimeout = 10 * time.Second
)

var _ = Describe("Metrics", func() {
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

	Context("When a metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeMetricsTestK8sObjects(portRegistry, "metric-mocks")

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

		It("Should have a running pipeline", func() {
			metricPipelineShouldBeRunning("test")
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

	Context("When reaching the pipeline limit", Ordered, func() {
		allPipelines := make(map[string][]client.Object, 0)

		BeforeAll(func() {
			for i := 0; i < maxNumberOfMetricPipelines; i++ {
				pipelineName := fmt.Sprintf("pipeline-%d", i)
				pipelineObjects := makeBrokenMetricPipeline(pipelineName)
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
				metricPipelineShouldBeRunning(pipelineName)
			}
		})

		It("Should have a pending pipeline", func() {
			By("Creating an additional pipeline", func() {
				newPipelineName := "new-pipeline"
				newPipeline := makeBrokenMetricPipeline(newPipelineName)
				allPipelines[newPipelineName] = newPipeline

				Expect(kitk8s.CreateObjects(ctx, k8sClient, newPipeline...)).Should(Succeed())
				metricPipelineShouldStayPending(newPipelineName)
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				deletedPipeline := allPipelines["pipeline-0"]
				delete(allPipelines, "pipeline-0")

				Expect(kitk8s.DeleteObjects(ctx, k8sClient, deletedPipeline...)).Should(Succeed())
				for pipelineName := range allPipelines {
					metricPipelineShouldBeRunning(pipelineName)
				}
			})
		})
	})

	Context("When a broken metricpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeMetricsTestK8sObjects(portRegistry, "metric-mocks-broken-pipeline")
			secondPipeline := makeBrokenMetricPipeline("pipeline-2")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, secondPipeline...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			metricPipelineShouldBeRunning("test")
			metricPipelineShouldBeRunning("pipeline-2")
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

	Context("When multiple metricpipelines exist", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeMultiPipelineMetricsTestK8sObjects(portRegistry, "metrics-mocks-multi-pipeline")

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			metricPipelineShouldBeRunning("pipeline-1")
			metricPipelineShouldBeRunning("pipeline-2")
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
					HaveNumberOfMetrics(2 * len(gauges))))) // Metrics should arrive in the backend twice
			}, timeout, interval).Should(Succeed())
		})
	})
})

func metricPipelineShouldBeRunning(pipelineName string) {
	Eventually(func(g Gomega) bool {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		return pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning)
	}, timeout, interval).Should(BeTrue())
}

func metricPipelineShouldStayPending(pipelineName string) {
	Consistently(func(g Gomega) {
		var pipeline telemetryv1alpha1.MetricPipeline
		key := types.NamespacedName{Name: pipelineName}
		g.Expect(k8sClient.Get(ctx, key, &pipeline)).To(Succeed())
		g.Expect(pipeline.Status.HasCondition(telemetryv1alpha1.MetricPipelineRunning)).To(BeFalse())
	}, tracePipelineReconciliationTimeout, interval).Should(Succeed())
}

// makeMetricsTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeMetricsTestK8sObjects(portRegistry testkit.PortRegistry, namespace string) []client.Object {
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
	mocksNamespace := kitk8s.NewNamespace(namespace)
	mockBackend := mocks.NewBackend("metric-receiver", mocksNamespace.Name(), "/metrics/"+telemetryDataFilename, mocks.SignalTypeMetrics)
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

// makeMultiPipelineMetricsTestK8sObjects returns the list of mandatory E2E test suite k8s objects including two metricpipelines.
func makeMultiPipelineMetricsTestK8sObjects(portRegistry testkit.PortRegistry, namespace string) []client.Object {
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
	mocksNamespace := kitk8s.NewNamespace(namespace)
	mockBackend := mocks.NewBackend("metric-receiver", mocksNamespace.Name(), "/metrics/"+telemetryDataFilename, mocks.SignalTypeMetrics)
	mockBackendConfigMap := mockBackend.ConfigMap("metric-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPortMapping("http-web", httpWebPort, httpWebNodePort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)

	hostSecret1 := kitk8s.NewOpaqueSecret("metric-rcv-hostname-1", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline1 := kitmetric.NewPipeline("pipeline-1", hostSecret1.SecretKeyRef("metric-host"))

	hostSecret2 := kitk8s.NewOpaqueSecret("metric-rcv-hostname-2", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline2 := kitmetric.NewPipeline("pipeline-2", hostSecret2.SecretKeyRef("metric-host"))

	// Kyma-system namespace objects.
	metricGatewayExternalService := kitk8s.NewService("telemetry-otlp-metrics-external", kymaSystemNamespaceName).
		WithPortMapping("grpc-otlp", grpcOTLPPort, grpcOTLPNodePort).
		WithPortMapping("http-metrics", httpMetricsPort, httpMetricsNodePort)

	return []client.Object{
		mocksNamespace.K8sObject(),
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		hostSecret1.K8sObject(),
		metricPipeline1.K8sObject(),
		hostSecret2.K8sObject(),
		metricPipeline2.K8sObject(),
		metricGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", metricGatewayBaseName)),
	}
}

func makeBrokenMetricPipeline(name string) []client.Object {
	hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
	metricPipeline := kitmetric.NewPipeline(name, hostSecret.SecretKeyRef("metric-host"))

	return []client.Object{
		hostSecret.K8sObject(),
		metricPipeline.K8sObject(),
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
