//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/collector/pdata/pmetric"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	"github.com/kyma-project/telemetry-manager/test/testkit"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	kitmetric "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/metric"
	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/urlprovider"
	kitmetrics "github.com/kyma-project/telemetry-manager/test/testkit/otlp/metrics"
)

var (
	metricGatewayBaseName      = "telemetry-metric-gateway"
	maxNumberOfMetricPipelines = 3
)

var _ = Describe("Metrics", Label("metrics"), func() {
	Context("When a metricpipeline exists", Ordered, func() {
		var (
			pipelines       *kyma.PipelineList
			urls            *urlprovider.URLProvider
			mockBackendName = "metric-receiver"
			mockNs          = "metric-mocks"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsTestK8sObjects(
				mockNs,
				backend.NewOptions(mockBackendName),
			)
			pipelines = pipelinesProvider
			urls = urlProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", Label(operationalTest), func() {
			deploymentShouldBeReady(metricGatewayBaseName, kymaSystemNamespaceName)
		})

		It("Should have 2 metric gateway replicas", func() {
			Eventually(func(g Gomega) int32 {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
				err := k8sClient.Get(ctx, key, &deployment)
				g.Expect(err).NotTo(HaveOccurred())
				return *deployment.Spec.Replicas
			}, timeout, interval).Should(Equal(int32(2)))
		})

		It("Should have a metrics backend running", Label(operationalTest), func() {
			deploymentShouldBeReady(mockBackendName, mockNs)
		})

		It("Should be able to get metric gateway metrics endpoint", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.Metrics())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running pipeline", Label(operationalTest), func() {
			metricPipelineShouldBeRunning(pipelines.First())
		})

		It("Should verify end-to-end metric delivery", Label(operationalTest), func() {
			gauges := makeAndSendGaugeMetrics(urls.OTLPPush())
			metricsShouldBeDelivered(urls.MockBackendExport(mockBackendName), gauges)
		})

		It("Should have a working network policy", func() {
			var networkPolicy networkingv1.NetworkPolicy
			key := types.NamespacedName{Name: metricGatewayBaseName + "-pprof-deny-ingress", Namespace: kymaSystemNamespaceName}
			Expect(k8sClient.Get(ctx, key, &networkPolicy)).To(Succeed())

			Eventually(func(g Gomega) {
				var podList corev1.PodList
				g.Expect(k8sClient.List(ctx, &podList, client.InNamespace(kymaSystemNamespaceName), client.MatchingLabels{"app.kubernetes.io/name": metricGatewayBaseName})).To(Succeed())
				g.Expect(podList.Items).NotTo(BeEmpty())

				metricGatewayPodName := podList.Items[0].Name
				pprofEndpoint := proxyClient.ProxyURLForPod(kymaSystemNamespaceName, metricGatewayPodName, "debug/pprof/", ports.Pprof)

				resp, err := proxyClient.Get(pprofEndpoint)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusServiceUnavailable))
			}, timeout, interval).Should(Succeed())
		})

	})

	Context("When a MetricPipeline has ConvertToDelta flag active", Ordered, func() {
		var (
			pipelines       *kyma.PipelineList
			urls            *urlprovider.URLProvider
			mockBackendName = "metric-receiver"
			mockNs          = "metric-mocks-delta"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsTestK8sObjects(
				mockNs,
				backend.NewOptions(mockBackendName),
			)
			pipelines = pipelinesProvider
			urls = urlProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a running metric gateway deployment", func() {
			deploymentShouldBeReady(metricGatewayBaseName, kymaSystemNamespaceName)

		})

		It("Should have a metrics backend running", func() {
			deploymentShouldBeReady(mockBackendName, mockNs)
		})

		It("Should have a running pipeline", func() {
			metricPipelineShouldBeRunning(pipelines.First())
		})

		It("Should verify end-to-end metric delivery", func() {
			cumulativeSums := makeAndSendSumMetrics(urls.OTLPPush())
			for i := 0; i < len(cumulativeSums); i++ {
				cumulativeSums[i].Sum().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
			}
			metricsShouldBeDelivered(urls.MockBackendExport(mockBackendName), cumulativeSums)
		})
	})

	Context("When metricpipeline with missing secret reference exists", Ordered, func() {
		hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("metric-host", "http://localhost:4317"))
		metricPipeline := kitmetric.NewPipeline("without-secret", hostSecret.SecretKeyRef("metric-host"))

		BeforeAll(func() {
			Expect(kitk8s.CreateObjects(ctx, k8sClient, metricPipeline.K8sObject())).Should(Succeed())

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, metricPipeline.K8sObject(), hostSecret.K8sObject())).Should(Succeed())
			})
		})

		It("Should have pending metricpipeline", func() {
			metricPipelineShouldStayPending(metricPipeline.Name())
		})

		It("Should not have metric-gateway deployment", func() {
			Consistently(func(g Gomega) {
				var deployment appsv1.Deployment
				key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
				g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
			}, reconciliationTimeout, interval).ShouldNot(Succeed())
		})

		It("Should have running metricpipeline", func() {
			By("Creating missing secret", func() {
				Expect(kitk8s.CreateObjects(ctx, k8sClient, hostSecret.K8sObject())).Should(Succeed())
			})

			metricPipelineShouldBeRunning(metricPipeline.Name())
		})
	})

	Context("When reaching the pipeline limit", Ordered, func() {
		pipelinesObjects := make(map[string][]client.Object, 0)
		pipelines := kyma.NewPipelineList()

		BeforeAll(func() {
			for i := 0; i < maxNumberOfMetricPipelines; i++ {
				objs, name := makeBrokenMetricPipeline(fmt.Sprintf("pipeline-%d", i))
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
				metricPipelineShouldBeRunning(pipeline)
				metricPipelineShouldBeDeployed(pipeline)
			}
		})

		It("Should have a pending pipeline", func() {
			By("Creating an additional pipeline", func() {
				newObjs, newName := makeBrokenMetricPipeline("exceeding-pipeline")
				pipelinesObjects[newName] = newObjs
				pipelines.Append(newName)

				Expect(kitk8s.CreateObjects(ctx, k8sClient, newObjs...)).Should(Succeed())
				metricPipelineShouldStayPending(newName)
				metricPipelineShouldNotBeDeployed(newName)
			})
		})

		It("Should have only running pipeline", func() {
			By("Deleting a pipeline", func() {
				deletedPipeline := pipelinesObjects[pipelines.First()]
				delete(pipelinesObjects, pipelines.First())

				Expect(kitk8s.DeleteObjects(ctx, k8sClient, deletedPipeline...)).Should(Succeed())
				for _, pipeline := range pipelines.All()[1:] {
					metricPipelineShouldBeRunning(pipeline)
				}
			})
		})
	})

	Context("When a broken metricpipeline exists", Ordered, func() {
		var (
			brokenPipelineName string
			pipelines          *kyma.PipelineList
			urls               *urlprovider.URLProvider
			mockBackendName    = "metric-receiver"
			mockNs             = "metric-mocks-broken-pipeline"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsTestK8sObjects(
				mockNs,
				backend.NewOptions(mockBackendName),
			)
			pipelines = pipelinesProvider
			urls = urlProvider
			brokenPipelineObjs, brokenName := makeBrokenMetricPipeline("pipeline-1")
			brokenPipelineName = brokenName

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, brokenPipelineObjs...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			Expect(kitk8s.CreateObjects(ctx, k8sClient, brokenPipelineObjs...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			metricPipelineShouldBeRunning(pipelines.First())
			metricPipelineShouldBeRunning(brokenPipelineName)
		})

		It("Should have a running metric gateway deployment", func() {
			deploymentShouldBeReady(metricGatewayBaseName, kymaSystemNamespaceName)

		})
		It("Should have a metrics backend running", func() {
			deploymentShouldBeReady(mockBackendName, mockNs)
		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := makeAndSendGaugeMetrics(urls.OTLPPush())
			metricsShouldBeDelivered(urls.MockBackendExport(mockBackendName), gauges)
		})
	})

	Context("When multiple metricpipelines exist", Ordered, func() {
		var (
			pipelines                   *kyma.PipelineList
			urls                        *urlprovider.URLProvider
			primaryMockDeploymentName   = "metric-receiver"
			auxiliaryMockDeploymentName = "metric-receiver-1"
			mockNs                      = "metric-mocks-multi-pipeline"
		)

		BeforeAll(func() {
			k8sObjects, urlProvider, pipelinesProvider := makeMetricsTestK8sObjects(
				mockNs,
				backend.NewOptions(primaryMockDeploymentName),
				backend.NewOptions(auxiliaryMockDeploymentName),
			)
			pipelines = pipelinesProvider
			urls = urlProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			metricPipelineShouldBeRunning(pipelines.First())
			metricPipelineShouldBeRunning(pipelines.Second())
		})

		It("Should have a running metric gateway deployment", func() {
			deploymentShouldBeReady(metricGatewayBaseName, kymaSystemNamespaceName)

		})

		It("Should have a metrics backend running", func() {
			deploymentShouldBeReady(primaryMockDeploymentName, mockNs)
			deploymentShouldBeReady(auxiliaryMockDeploymentName, mockNs)

		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := makeAndSendGaugeMetrics(urls.OTLPPush())
			metricsShouldBeDelivered(urls.MockBackendExport(primaryMockDeploymentName), gauges)
			metricsShouldBeDelivered(urls.MockBackendExport(auxiliaryMockDeploymentName), gauges)
		})
	})

	Context("When a metricpipeline with TLS activated exists", Ordered, func() {
		var (
			pipelines       *kyma.PipelineList
			urls            *urlprovider.URLProvider
			mockBackendName = "metric-tls-receiver"
			mockNs          = "metric-mocks-tls-pipeline"
		)

		BeforeAll(func() {
			k8sObjects, metricsURLProvider, pipelinesProvider := makeMetricsTestK8sObjects(
				mockNs,
				backend.NewOptions(
					mockBackendName,
					backend.WithTLS(),
				),
			)
			pipelines = pipelinesProvider
			urls = metricsURLProvider

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have running pipelines", func() {
			metricPipelineShouldBeRunning(pipelines.First())
		})

		It("Should have a metric backend running", func() {
			deploymentShouldBeReady(mockBackendName, mockNs)
		})

		It("Should verify end-to-end metric delivery", func() {
			gauges := makeAndSendGaugeMetrics(urls.OTLPPush())
			metricsShouldBeDelivered(urls.MockBackendExport(mockBackendName), gauges)
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
	}, reconciliationTimeout, interval).Should(Succeed())
}

func metricPipelineShouldBeDeployed(pipelineName string) {
	Eventually(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
		g.Expect(k8sClient.Get(ctx, key, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return strings.Contains(configString, pipelineAlias)
	}, timeout, interval).Should(BeTrue())
}

func metricPipelineShouldNotBeDeployed(pipelineName string) {
	Consistently(func(g Gomega) bool {
		var collectorConfig corev1.ConfigMap
		key := types.NamespacedName{Name: metricGatewayBaseName, Namespace: kymaSystemNamespaceName}
		g.Expect(k8sClient.Get(ctx, key, &collectorConfig)).To(Succeed())
		configString := collectorConfig.Data["relay.conf"]
		pipelineAlias := fmt.Sprintf("otlp/%s", pipelineName)
		return !strings.Contains(configString, pipelineAlias)
	}, reconciliationTimeout, interval).Should(BeTrue())
}

// makeMetricsTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeMetricsTestK8sObjects(namespace string, backendOptions ...*backend.Options) ([]client.Object, *urlprovider.URLProvider, *kyma.PipelineList) {
	var (
		objs      []client.Object
		pipelines = kyma.NewPipelineList()
		urls      = urlprovider.New()
	)

	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, mocksNamespace.K8sObject())

	// new backend
	for _, options := range backendOptions {
		options.WithPersistentHostSecret = isOperational()
		options.SignalType = backend.SignalTypeMetrics

		if options.WithTLS {
			backendDNSName := fmt.Sprintf("%s.%s.svc.cluster.local", options.Name, mocksNamespace.Name())
			certs, err := testkit.GenerateTLSCerts(backendDNSName)
			Expect(err).NotTo(HaveOccurred())
			options.TLSCerts = certs
			options.MetricPipelineOptions = append(options.MetricPipelineOptions, getTLSConfigMetricPipelineOption(
				certs.CaCertPem.String(), certs.ClientCertPem.String(), certs.ClientKeyPem.String()),
			)
		}

		// Mocks namespace objects.
		mockBackend := backend.New(mocksNamespace.Name(), options)

		// Default namespace objects.
		metricPipeline := kitmetric.NewPipeline(fmt.Sprintf("%s-%s", options.Name, "pipeline"),
			mockBackend.GetHostSecretRefKey()).Persistent(isOperational())
		pipelines.Append(metricPipeline.Name())

		objs = append(objs, mockBackend.K8sObjects()...)
		objs = append(objs, metricPipeline.K8sObject(options.MetricPipelineOptions...))

		urls.SetMockBackendExport(options.Name, proxyClient.ProxyURLForService(
			namespace, options.Name, backend.TelemetryDataFilename, backend.HTTPWebPort),
		)
	}

	urls.SetOTLPPush(proxyClient.ProxyURLForService(
		kymaSystemNamespaceName, "telemetry-otlp-metrics", "v1/metrics/", ports.OTLPHTTP),
	)

	// Kyma-system namespace objects.
	metricGatewayExternalService := kitk8s.NewService("telemetry-otlp-metrics-external", kymaSystemNamespaceName).
		WithPort("grpc-otlp", ports.OTLPGRPC).
		WithPort("http-metrics", ports.Metrics)
	urls.SetMetrics(proxyClient.ProxyURLForService(
		kymaSystemNamespaceName, "telemetry-otlp-metrics-external", "metrics", ports.Metrics),
	)

	objs = append(objs, metricGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", metricGatewayBaseName)))

	return objs, urls, pipelines
}

func makeBrokenMetricPipeline(name string) ([]client.Object, string) {
	hostSecret := kitk8s.NewOpaqueSecret("metric-rcv-hostname-"+name, defaultNamespaceName, kitk8s.WithStringData("metric-host", "http://unreachable:4317"))
	metricPipeline := kitmetric.NewPipeline(name, hostSecret.SecretKeyRef("metric-host"))

	return []client.Object{
		hostSecret.K8sObject(),
		metricPipeline.K8sObject(),
	}, metricPipeline.Name()
}

func getCumulativeToDeltaConversionMetricPipelineOption() kitmetric.PipelineOption {
	return func(metricPipeline telemetryv1alpha1.MetricPipeline) {
		metricPipeline.Spec.Output.ConvertToDelta = true
	}
}

func getTLSConfigMetricPipelineOption(caCertPem, clientCertPem, clientKeyPem string) kitmetric.PipelineOption {
	return func(metricPipeline telemetryv1alpha1.MetricPipeline) {
		metricPipeline.Spec.Output.Otlp.TLS = &telemetryv1alpha1.OtlpTLS{
			Insecure:           false,
			InsecureSkipVerify: false,
			CA: telemetryv1alpha1.ValueType{
				Value: caCertPem,
			},
			Cert: telemetryv1alpha1.ValueType{
				Value: clientCertPem,
			},
			Key: telemetryv1alpha1.ValueType{
				Value: clientKeyPem,
			},
		}
	}
}

func makeAndSendGaugeMetrics(otlpPushURL string) []pmetric.Metric {
	builder := kitmetrics.NewBuilder()
	var gauges []pmetric.Metric
	for i := 0; i < 50; i++ {
		gauge := kitmetrics.NewGauge()
		gauges = append(gauges, gauge)
		builder.WithMetric(gauge)
	}
	Expect(sendGaugeMetrics(context.Background(), builder.Build(), otlpPushURL)).To(Succeed())

	return gauges
}

func sendGaugeMetrics(ctx context.Context, metrics pmetric.Metrics, otlpPushURL string) error {
	sender, err := kitmetrics.NewHTTPExporter(otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}
	return sender.ExportGaugeMetrics(ctx, metrics)
}

func makeAndSendSumMetrics(otlpPushURL string) []pmetric.Metric {
	builder := kitmetrics.NewBuilder()
	var cumulativeSums []pmetric.Metric

	for i := 0; i < 50; i++ {
		sum := kitmetrics.NewCumulativeSum()
		cumulativeSums = append(cumulativeSums, sum)
		builder.WithMetric(sum)
	}
	Expect(sendSumMetrics(context.Background(), builder.Build(), otlpPushURL)).To(Succeed())

	return cumulativeSums
}

func sendSumMetrics(ctx context.Context, metrics pmetric.Metrics, otlpPushURL string) error {
	sender, err := kitmetrics.NewHTTPExporter(otlpPushURL, proxyClient)
	if err != nil {
		return fmt.Errorf("unable to create an OTLP HTTP Metric Exporter instance: %w", err)
	}
	return sender.ExportSumMetrics(ctx, metrics)
}

func metricsShouldBeDelivered(proxyURL string, metrics []pmetric.Metric) {
	Eventually(func(g Gomega) {
		resp, err := proxyClient.Get(proxyURL)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
		g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
			ConsistOfNumberOfMetrics(len(metrics)),
			ContainMetrics(metrics...))))
		err = resp.Body.Close()
		g.Expect(err).NotTo(HaveOccurred())
	}, timeout, interval).Should(Succeed())
}

func suffixize(name string, idx int) string {
	if idx == 0 {
		return name
	}

	return fmt.Sprintf("%s-%d", name, idx)
}
