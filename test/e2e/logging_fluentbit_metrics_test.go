//go:build e2edisabled

package e2e

import (
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/testkit/kyma/telemetry/log"

	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/backend"

	. "github.com/kyma-project/telemetry-manager/test/testkit/matchers"
)

var (
	telemetryFluentbitName              = "telemetry-fluent-bit"
	telemetryWebhookEndpoint            = "telemetry-operator-webhook"
	telemetryFluentbitMetricServiceName = "telemetry-fluent-bit-metrics"
)

var _ = Describe("Logging", Label("logging"), func() {
	Context("When a logpipeline exists", Ordered, func() {
		var (
			mockNs             = "log-pipeline-mocks"
			mockDeploymentName = "log-receiver"
		)

		BeforeAll(func() {
			k8sObjects := makeLogsTestK8sObjects(mockNs, mockDeploymentName)
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
				g.Expect(resp).To(HaveHTTPBody(SatisfyAll(
					ContainPrometheusMetric("fluentbit_uptime"))))
			}, timeout, interval).Should(Succeed())
		})
	})
})

func makeLogsTestK8sObjects(mockNs string, mockDeploymentName string) []client.Object {
	var (
		objs []client.Object
	)
	objs = append(objs, kitk8s.NewNamespace(mockNs).K8sObject())

	//// Mocks namespace objects.
	mockBackend := backend.New(mockDeploymentName, mockNs, backend.SignalTypeLogs)
	objs = append(objs, mockBackend.K8sObjects()...)

	// Default namespace objects.
	logPipeline := kitlog.NewPipeline("pipeline-mock-backend").WithStdout()
	objs = append(objs, logPipeline.K8sObject())

	return objs
}
