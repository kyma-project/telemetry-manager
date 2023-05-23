//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Telemetry-manager", func() {
	Context("After deploying manifest", func() {
		It("Should have kyma-system namespace", func() {
			var namespace corev1.Namespace
			key := types.NamespacedName{
				Name: kymaSystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a running manager deployment", func() {
			var deployment appsv1.Deployment
			key := types.NamespacedName{
				Name:      "telemetry-operator",
				Namespace: kymaSystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &deployment)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
					Namespace:     kymaSystemNamespaceName,
				}
				var pods corev1.PodList
				err := k8sClient.List(ctx, &pods, &listOptions)
				Expect(err).NotTo(HaveOccurred())
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

		It("Should have a webhook service", func() {
			var service corev1.Service
			key := types.NamespacedName{
				Name:      "telemetry-operator-webhook",
				Namespace: kymaSystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &service)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []corev1.EndpointAddress {
				var endpoints corev1.Endpoints
				err := k8sClient.Get(ctx, key, &endpoints)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints.Subsets).NotTo(BeEmpty())
				return endpoints.Subsets[0].Addresses
			}, timeout, interval).ShouldNot(BeEmpty())
		})

		It("Should have a metrics service", func() {
			var service corev1.Service
			key := types.NamespacedName{
				Name:      "telemetry-operator-metrics",
				Namespace: kymaSystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &service)
			Expect(err).NotTo(HaveOccurred())

			Expect(service.Annotations).Should(HaveKeyWithValue("prometheus.io/scrape", "true"))
			Expect(service.Annotations).Should(HaveKeyWithValue("prometheus.io/port", "8080"))

			Eventually(func() []corev1.EndpointAddress {
				var endpoints corev1.Endpoints
				err := k8sClient.Get(ctx, key, &endpoints)
				Expect(err).NotTo(HaveOccurred())
				return endpoints.Subsets[0].Addresses
			}, timeout, interval).ShouldNot(BeEmpty())
		})

		It("Should have LogPipelines CRD", func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "logpipelines.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have LogParsers CRD", func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "logparsers.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have TracePipelines CRD", func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "tracepipelines.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have MetricPipelines CRD", func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "metricpipelines.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
