//go:build e2e

package e2e

import (
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
	kitlog "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/log"
	"github.com/kyma-project/telemetry-manager/test/e2e/testkit/mocks"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	telemetryFluentbitName   = "telemetry-fluent-bit"
	telemetryWebhookEndpoint = "telemetry-operator-webhook"
)

var _ = Describe("Logging", func() {
	Context("When a logpipeline exists", Ordered, func() {
		var (
			mockNs             = "log-mocks-single-pipeline"
			mockDeploymentName = "log-receiver"
		)

		BeforeAll(func() {
			k8sObjects, _ := makeLogsTestK8sObjects(mockNs, mockDeploymentName)

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

		grpcOTLPPort    = 4317
		httpMetricsPort = 8888
		httpOTLPPort    = 4318
		httpWebPort     = 80
	)
	mocksNamespace := kitk8s.NewNamespace(namespace)
	objs = append(objs, kitk8s.NewNamespace(namespace).K8sObject())

	//// Mocks namespace objects.
	mockBackend := mocks.NewLogBackend(mockDeploymentName, mocksNamespace.Name(), "/logs/"+telemetryDataFilename)

	mockBackendConfigMap := mockBackend.LogConfigMap("log-receiver-config")
	mockBackendDeployment := mockBackend.LogDeployment(mockBackendConfigMap.Name())
	mockBackendExternalService := mockBackend.ExternalService().
		WithPort("grpc-otlp", grpcOTLPPort).
		WithPort("http-otlp", httpOTLPPort).
		WithPort("http-web", httpWebPort)

	// Default namespace objects.
	otlpEndpointURL := mockBackendExternalService.OTLPEndpointURL(grpcOTLPPort)
	hostSecret := kitk8s.NewOpaqueSecret("log-rcv-hostname", defaultNamespaceName, kitk8s.WithStringData("log-host", otlpEndpointURL))
	logPipeline := kitlog.NewLogPipeline("pipeline", hostSecret.SecretKeyRef("log-host"))

	objs = append(objs, []client.Object{
		mockBackendConfigMap.K8sObject(),
		mockBackendDeployment.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		mockBackendExternalService.K8sObject(kitk8s.WithLabel("app", mockBackend.Name())),
		hostSecret.K8sObject(),
		logPipeline.K8sObject(),
	}...)

	urls.SetMockBackendExportAt(proxyClient.ProxyURLForService(mocksNamespace.Name(), mockBackend.Name(), telemetryDataFilename, httpWebPort), 0)
	urls.SetOTLPPush(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-logs", "v1/traces/", httpOTLPPort))

	// Kyma-system namespace objects.
	logGatewayExternalService := kitk8s.NewService("telemetry-otlp-logs-external", kymaSystemNamespaceName).
		WithPort("http-input", grpcOTLPPort).
		WithPort("http-metrics", httpMetricsPort)
	urls.SetMetrics(proxyClient.ProxyURLForService(kymaSystemNamespaceName, "telemetry-otlp-logs-external", "metrics", httpMetricsPort))

	objs = append(objs, logGatewayExternalService.K8sObject(kitk8s.WithLabel("app.kubernetes.io/name", "telemetry-log-collector")))

	return objs, urls
}
