//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s/verifiers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/matchers"

	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kitmetric "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/metric"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/metrics"
)

type metricsURLProvider struct {
	otlpPush                 string
	metrics                  string
	mockBackendMetricsExport string
}

var (
	metricGatewayBaseName               = "telemetry-metric-gateway"
	maxNumberOfMetricPipelines          = 3
	metricPipelineReconciliationTimeout = 10 * time.Second
)

var _ = Describe("Metrics", func() {
	Context("When a metricpipeline exists", Ordered, func() {
		var (
			urls               metricsURLProvider
			mockDeploymentName = "metric-receiver"
			mockNs             = "metric-mocks"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider := makeMetricsTestK8sObjects(mockNs, mockDeploymentName)
			urls = urlProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a metrics backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should be able to get metric gateway metrics endpoint", func() {
			Eventually(func(g Gomega) {
				resp, err := httpsAuthProvider.Get(urls.metrics)
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
			Expect(sendMetrics(context.Background(), builder.Build(), urls.otlpPush)).To(Succeed())

			Eventually(func(g Gomega) {
				resp, err := httpsAuthProvider.Get(urls.mockBackendMetricsExport)
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
		var (
			urls               metricsURLProvider
			mockDeploymentName = "metric-receiver"
			mockNs             = "metric-mocks-broken-pipeline"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider := makeMetricsTestK8sObjects(mockNs, mockDeploymentName)
			urls = urlProvider
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

		It("Should have a running metric gateway deployment", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
		It("Should have a metrics backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
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
			Expect(sendMetrics(context.Background(), builder.Build(), urls.otlpPush)).Should(Succeed())

			Eventually(func(g Gomega) {
				resp, err := httpsAuthProvider.Get(urls.mockBackendMetricsExport)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					HaveMetrics(gauges...))))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When multiple metricpipelines exist", Ordered, func() {
		var (
			urls               metricsURLProvider
			mockDeploymentName = "metric-receiver"
			mockNs             = "metric-mocks-multi-pipeline"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider := makeMultiPipelineMetricsTestK8sObjects(mockNs, mockDeploymentName)
			urls = urlProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			metricPipelineShouldBeRunning("pipeline-1")
			metricPipelineShouldBeRunning("pipeline-2")
		})

		It("Should have a running metric gateway deployment", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a metrics backend running", func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
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
			Expect(sendMetrics(context.Background(), builder.Build(), urls.otlpPush)).To(Succeed())

			Eventually(func(g Gomega) {
				resp, err := httpsAuthProvider.Get(urls.mockBackendMetricsExport)
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
	}, metricPipelineReconciliationTimeout, interval).Should(Succeed())
}

// makeMetricsTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeMetricsTestK8sObjects(namespace string, mockDeploymentName string) ([]client.Object, metricsURLProvider) {
	var (
		grpcOTLPPort    = 4317
		httpMetricsPort = 8888
		httpOTLPPort    = 4318
		httpWebPort     = 80
	)

	// Mocks namespace objects.
	mocksNamespace := kitk8s.NewNamespace(namespace)
	mockBackend := mocks.NewBackend(mockDeploymentName, mocksNamespace.Name(), "/metrics/"+telemetryDataFilename, mocks.SignalTypeMetrics)
	mockBackendConfigMap := mockBackend.ConfigMap("metric-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline := kitmetric.NewPipeline("test", hostSecret.SecretKeyRef("metric-host"))

	// Kyma-system namespace objects.
	metricGatewayExternalService := kitk8s.NewService("telemetry-otlp-metrics-external", kymaSystemNamespaceName).
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-metrics", httpMetricsPort)

	return []client.Object{
			mocksNamespace.K8sObject(),
			mockBackendConfigMap.K8sObject(),
			mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
			hostSecret.K8sObject(),
			metricPipeline.K8sObject(),
			metricGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", metricGatewayBaseName)),
		}, metricsURLProvider{
			otlpPush:                 httpsAuthProvider.URL(mocksNamespace.Name(), mockBackend.Name(), "v1/metrics/", httpOTLPPort),
			metrics:                  httpsAuthProvider.URL(kymaSystemNamespaceName, "telemetry-otlp-metrics-external", "metrics", httpMetricsPort),
			mockBackendMetricsExport: httpsAuthProvider.URL(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort),
		}
}

// makeMultiPipelineMetricsTestK8sObjects returns the list of mandatory E2E test suite k8s objects including two metricpipelines.
func makeMultiPipelineMetricsTestK8sObjects(namespace string, mockDeploymentName string) ([]client.Object, metricsURLProvider) {
	var (
		grpcOTLPPort    = 4317
		httpMetricsPort = 8888
		httpOTLPPort    = 4318
		httpWebPort     = 80
	)

	// Mocks namespace objects.
	mocksNamespace := kitk8s.NewNamespace(namespace)
	mockBackend := mocks.NewBackend(mockDeploymentName, mocksNamespace.Name(), "/metrics/"+telemetryDataFilename, mocks.SignalTypeMetrics)
	mockBackendConfigMap := mockBackend.ConfigMap("metric-receiver-config")
	mockBackendDeployment := mockBackend.Deployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)

	hostSecret1 := kitk8s.NewOpaqueSecret("metric-rcv-hostname-1", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline1 := kitmetric.NewPipeline("pipeline-1", hostSecret1.SecretKeyRef("metric-host"))

	hostSecret2 := kitk8s.NewOpaqueSecret("metric-rcv-hostname-2", defaultNamespaceName, kitk8s.WithStringData("metric-host", otlpEndpointURL))
	metricPipeline2 := kitmetric.NewPipeline("pipeline-2", hostSecret2.SecretKeyRef("metric-host"))

	// Kyma-system namespace objects.
	metricGatewayExternalService := kitk8s.NewService("telemetry-otlp-metrics-external", kymaSystemNamespaceName).
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-metrics", httpMetricsPort)

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
		}, metricsURLProvider{
			otlpPush:                 httpsAuthProvider.URL(mocksNamespace.Name(), mockBackend.Name(), "v1/metrics/", httpOTLPPort),
			metrics:                  httpsAuthProvider.URL(kymaSystemNamespaceName, "telemetry-otlp-metrics-external", "metrics", httpMetricsPort),
			mockBackendMetricsExport: httpsAuthProvider.URL(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort),
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

func sendMetrics(ctx context.Context, metrics pmetric.Metrics, otlpPushURL string) error {
	sender, err := kitmetrics.NewHTTPExporter(otlpPushURL, httpsAuthProvider)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}
	return sender.Export(ctx, metrics)
}
