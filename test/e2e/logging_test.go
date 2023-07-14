//go:build e2e

package e2e

import (
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s/verifiers"
	kitlog "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/kyma-project/telemetry-manager/test/e2e/testkit/otlp/matchers"
)

var (
	telemetryFluentbitName              = "telemetry-fluent-bit"
	telemetryWebhookEndpoint            = "telemetry-operator-webhook"
	telemetryFluentbitMetricServiceName = "telemetry-fluent-bit-metrics"
)

var _ = Describe("Logging", func() {
	Context("When a logpipeline exists", Ordered, func() {
		var (
			urls               *mocks.URLProvider
			mockNs             = "log-mocks-single-pipeline"
			mockDeploymentName = "log-receiver"
		)

		BeforeAll(func() {
			k8sObjects, logsURLProvider := makeLogsTestK8sObjects(mockNs, mockDeploymentName)
			urls = logsURLProvider
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, k8sObjects...)).Should(Succeed())
		})

		It("Should have a healthy webhook", func() {
			Eventually(func(g Gomega) {
				var endPoint corev1.Endpoints
				key := types.NamespacedName{Name: telemetryWebhookEndpoint, Namespace: kymaSystemNamespaceName}
				g.Expect(k8sClient.Get(ctx, key, &endPoint)).To(Succeed())
				g.Expect(endPoint.Subsets).NotTo(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a running fluent-bit daemonset", func() {
			Eventually(func(g Gomega) bool {
				var daemonSet appsv1.DaemonSet
				key := types.NamespacedName{Name: telemetryFluentbitName, Namespace: kymaSystemNamespaceName}
				g.Expect(k8sClient.Get(ctx, key, &daemonSet)).To(Succeed())

				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(daemonSet.Spec.Selector.MatchLabels),
					Namespace:     kymaSystemNamespaceName,
				}
				var pods corev1.PodList
				g.Expect(k8sClient.List(ctx, &pods, &listOptions)).To(Succeed())
				for _, pod := range pods.Items {
					for _, containerStatus := range pod.Status.ContainerStatuses {
						g.Expect(containerStatus.State.Running).NotTo(BeNil())
					}
				}

				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should be able to get fluent-bit metrics endpoint", Label(operationalTest), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(proxyClient.ProxyURLForService("kyma-system", telemetryFluentbitMetricServiceName, "/metrics", 2020))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(SatisfyAll(
					HasValidPrometheusMetric("fluentbit_uptime")))))
			}, timeout, interval).Should(Succeed())
		})

		It("Should have a log backend running", Label("operational"), func() {
			Eventually(func(g Gomega) {
				key := types.NamespacedName{Name: mockDeploymentName, Namespace: mockNs}
				ready, err := verifiers.IsDeploymentReady(ctx, k8sClient, key)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ready).To(BeTrue())
			}, timeout*2, interval).Should(Succeed())
		})

		It("Should verify end-to-end log delivery", Label("operational"), func() {
			Eventually(func(g Gomega) {
				resp, err := proxyClient.Get(urls.MockBackendExport())
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp).To(HaveHTTPStatus(http.StatusOK))
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainLogs())))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("Handling optional loki logpipeline", Ordered, func() {
		It("Should have a running loki logpipeline", func() {
			By("Creating a loki service", func() {
				lokiService := makeLokiService()
				Expect(kitk8s.CreateObjects(ctx, k8sClient, lokiService)).Should(Succeed())

				Eventually(func(g Gomega) {
					var lokiLogPipeline telemetryv1alpha1.LogPipeline
					key := types.NamespacedName{Name: "loki"}
					g.Expect(k8sClient.Get(ctx, key, &lokiLogPipeline)).To(Succeed())
					g.Expect(lokiLogPipeline.Status.HasCondition(telemetryv1alpha1.LogPipelineRunning)).To(BeTrue())
				}, 2*time.Minute, interval).Should(Succeed())
			})
		})

		It("Should delete loki logpipeline", func() {
			By("Deleting loki service", func() {
				lokiService := makeLokiService()
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, lokiService)).Should(Succeed())

				Eventually(func(g Gomega) bool {
					var lokiLogPipeline telemetryv1alpha1.LogPipeline
					key := types.NamespacedName{Name: "loki"}
					err := k8sClient.Get(ctx, key, &lokiLogPipeline)
					return apierrors.IsNotFound(err)
				}, 2*time.Minute, interval).Should(BeTrue())
			})
		})
	})
})

func makeLokiService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "logging-loki",
			Namespace: kymaSystemNamespaceName,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     3100,
					Protocol: corev1.ProtocolTCP,
					Name:     "http-metrics",
				},
			},
		},
	}
}

func makeLogsTestK8sObjects(namespace string, mockDeploymentName string) ([]client.Object, *mocks.URLProvider) {
	var (
		objs []client.Object
		urls = mocks.NewURLProvider()

		grpcOTLPPort = 4317
		httpOTLPPort = 4318
		httpWebPort  = 80
		httpLogPort  = 9880
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, kitk8s.NewNamespace(namespace).K8sObject())

	//// Mocks namespace objects.
	mockHTTPBackend := mocks.NewHTTPBackend(mockDeploymentName, mocksNamespace.Name(), "/logs/"+telemetryDataFilename)

	mockBackendConfigMap := mockHTTPBackend.HTTPBackendConfigMap("log-receiver-config")
	mockFluentDConfigMap := mockHTTPBackend.FluentDConfigMap("log-receiver-config-fluentd")
	mockBackendDeployment := mockHTTPBackend.HTTPDeployment(mockBackendConfigMap.Name(), mockFluentDConfigMap.FluentDName())
	mockBackendExternalService := mockHTTPBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort).
		WithPort("http-log", httpLogPort)
	mockLogSpammer := mockHTTPBackend.LogSpammer()
	// Default namespace objects.
	logEndpointURL := mockBackendExternalService.Host()
	hostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("log-host", logEndpointURL))
	logHTTPPipeline := kitlog.NewHTTPPipeline("pipeline-mock-backend", hostSecret.SecretKeyRef("log-host"))

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockFluentDConfigMap.K8sObjectFluentDConfig(),
		mockBackendDeployment.K8sObjectHTTP(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockHTTPBackend.Name())),
		hostSecret.K8sObject(),
		logHTTPPipeline.K8sObjectHTTP(),
		mockLogSpammer.K8sObject(),
	}...)

	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockHTTPBackend.Name(), telemetryDataFilename, httpWebPort), 0)

	return objs, urls
}
