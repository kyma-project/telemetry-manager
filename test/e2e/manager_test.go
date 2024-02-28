//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

var _ = Describe("Telemetry Manager", func() {
	Context("After deploying manifest", func() {
		It("Should have kyma-system namespace", Label("telemetry"), func() {
			var namespace corev1.Namespace
			key := types.NamespacedName{
				Name: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a running manager deployment", Label("telemetry"), func() {
			var deployment appsv1.Deployment
			key := types.NamespacedName{
				Name:      "telemetry-manager",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &deployment)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
					Namespace:     kitkyma.SystemNamespaceName,
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
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue())
		})

		It("Should have a webhook service", Label("telemetry"), func() {
			var service corev1.Service
			key := types.NamespacedName{
				Name:      "telemetry-manager-webhook",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &service)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []corev1.EndpointAddress {
				var endpoints corev1.Endpoints
				err := k8sClient.Get(ctx, key, &endpoints)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints.Subsets).NotTo(BeEmpty())
				return endpoints.Subsets[0].Addresses
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(BeEmpty())
		})

		It("Should have a metrics service", Label("telemetry"), func() {
			var service corev1.Service
			key := types.NamespacedName{
				Name:      "telemetry-manager-metrics",
				Namespace: kitkyma.SystemNamespaceName,
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
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(BeEmpty())
		})

		It("Should have LogPipelines CRD", Label("logs"), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "logpipelines.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have LogParsers CRD", Label("logs"), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "logparsers.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have TracePipelines CRD", Label("traces"), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "tracepipelines.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have MetricPipelines CRD", Label("metrics"), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "metricpipelines.telemetry.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have Telemetry CRD", Label("telemetry"), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "telemetries.operator.kyma-project.io",
			}
			err := k8sClient.Get(ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.NamespaceScoped))
		})

		It("Should have a Busola extension for MetricPipelines CRD", Label("metrics"), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-metricpipelines",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a Busola extension for LogPipelines CRD", Label("logs"), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-logpipelines",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a Busola extension for TracePipelines CRD", Label("traces"), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-tracepipelines",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a Busola extension for Telemetry CRD", Label("telemetry"), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-module",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a NetworkPolicy", Label("telemetry"), func() {
			var networkPolicy networkingv1.NetworkPolicy
			key := types.NamespacedName{
				Name:      "telemetry-manager",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := k8sClient.Get(ctx, key, &networkPolicy)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have priority class resource created", Label("telemetry"), func() {
			priorityClassNames := []string{"telemetry-priority-class", "telemetry-priority-class-high"}
			var priorityClass schedulingv1.PriorityClass
			for _, prioClass := range priorityClassNames {
				key := types.NamespacedName{
					Name:      prioClass,
					Namespace: kitkyma.SystemNamespaceName,
				}
				err := k8sClient.Get(ctx, key, &priorityClass)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})
