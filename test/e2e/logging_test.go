//go:build e2e

package e2e

import (
	"time"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	kitk8s "github.com/kyma-project/telemetry-manager/test/e2e/testkit/k8s"
	kitlog "github.com/kyma-project/telemetry-manager/test/e2e/testkit/kyma/telemetry/log"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	telemetryFluentbitName   = "telemetry-fluent-bit"
	telemetryWebhookEndpoint = "telemetry-operator-webhook"
)

var _ = Describe("Logging", func() {
	Context("When a logpipeline exists", Ordered, func() {
		BeforeAll(func() {
			k8sObjects := makeLoggingTestK8sObjects()

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
				g.Expect(len(endPoint.Subsets)).NotTo(BeZero())
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
			By("Deleting loki service")
			lokiService := makeLokiService()
			Expect(kitk8s.DeleteObjects(ctx, k8sClient, lokiService)).Should(Succeed())

			Eventually(func(g Gomega) bool {
				var lokiLogPipeline telemetryv1alpha1.LogPipeline
				key := types.NamespacedName{Name: "loki"}
				err := k8sClient.Get(ctx, key, &lokiLogPipeline)
				if apierrors.IsNotFound(err) {
					return true
				}
				return false
			}, 2*time.Minute, interval).Should(BeTrue())
		})
	})
})

// makeLoggingTestK8sObjects returns the list of mandatory E2E test suite k8s objects.
func makeLoggingTestK8sObjects() []client.Object {
	logPipeline := kitlog.NewPipeline("test")
	return []client.Object{
		logPipeline.K8sObject(),
	}
}

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
