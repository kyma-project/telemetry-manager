//go:build e2e

package misc

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
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), func() {
	Context("After deploying manifest", func() {
		It("Should have kyma-system namespace", Label(LabelTelemetry), func() {
			var namespace corev1.Namespace
			key := types.NamespacedName{
				Name: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &namespace)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a running manager deployment", Label(LabelTelemetry), func() {
			var deployment appsv1.Deployment
			key := types.NamespacedName{
				Name:      "telemetry-manager",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &deployment)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				listOptions := client.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Selector.MatchLabels),
					Namespace:     kitkyma.SystemNamespaceName,
				}
				var pods corev1.PodList
				err := K8sClient.List(Ctx, &pods, &listOptions)
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

		It("Should have a webhook service", Label(LabelTelemetry), func() {
			var service corev1.Service
			key := types.NamespacedName{
				Name:      "telemetry-manager-webhook",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &service)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() []corev1.EndpointAddress {
				var endpoints corev1.Endpoints
				err := K8sClient.Get(Ctx, key, &endpoints)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoints.Subsets).NotTo(BeEmpty())
				return endpoints.Subsets[0].Addresses
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(BeEmpty())
		})

		It("Should have a metrics service", Label(LabelTelemetry), func() {
			var service corev1.Service
			err := K8sClient.Get(Ctx, kitkyma.TelemetryManagerMetricsServiceName, &service)
			Expect(err).NotTo(HaveOccurred())

			Expect(service.Annotations).Should(HaveKeyWithValue("prometheus.io/scrape", "true"))
			Expect(service.Annotations).Should(HaveKeyWithValue("prometheus.io/port", "8080"))

			Eventually(func() []corev1.EndpointAddress {
				var endpoints corev1.Endpoints
				err := K8sClient.Get(Ctx, kitkyma.TelemetryManagerMetricsServiceName, &endpoints)
				Expect(err).NotTo(HaveOccurred())
				return endpoints.Subsets[0].Addresses
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).ShouldNot(BeEmpty())
		})

		It("Should have LogPipelines CRD", Label(LabelLogs), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "logpipelines.telemetry.kyma-project.io",
			}
			err := K8sClient.Get(Ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have LogParsers CRD", Label(LabelLogs), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "logparsers.telemetry.kyma-project.io",
			}
			err := K8sClient.Get(Ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have TracePipelines CRD", Label(LabelTraces), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "tracepipelines.telemetry.kyma-project.io",
			}
			err := K8sClient.Get(Ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have MetricPipelines CRD", Label(LabelMetrics), Label(LabelSetA), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "metricpipelines.telemetry.kyma-project.io",
			}
			err := K8sClient.Get(Ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.ClusterScoped))
		})

		It("Should have Telemetry CRD", Label(LabelTelemetry), func() {
			var crd apiextensionsv1.CustomResourceDefinition
			key := types.NamespacedName{
				Name: "telemetries.operator.kyma-project.io",
			}
			err := K8sClient.Get(Ctx, key, &crd)
			Expect(err).NotTo(HaveOccurred())
			Expect(crd.Spec.Scope).To(Equal(apiextensionsv1.NamespaceScoped))
		})

		It("Should have a Busola extension for MetricPipelines CRD", Label(LabelMetrics), Label(LabelSetA), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-metricpipelines",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a Busola extension for LogPipelines CRD", Label(LabelLogs), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-logpipelines",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a Busola extension for TracePipelines CRD", Label(LabelTraces), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-tracepipelines",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a Busola extension for Telemetry CRD", Label(LabelTelemetry), func() {
			var cm corev1.ConfigMap
			key := types.NamespacedName{
				Name:      "telemetry-module",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &cm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have a NetworkPolicy", Label(LabelTelemetry), func() {
			var networkPolicy networkingv1.NetworkPolicy
			key := types.NamespacedName{
				Name:      "telemetry-manager",
				Namespace: kitkyma.SystemNamespaceName,
			}
			err := K8sClient.Get(Ctx, key, &networkPolicy)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should have priority class resource created", Label(LabelTelemetry), func() {
			priorityClassNames := []string{"telemetry-priority-class", "telemetry-priority-class-high"}
			var priorityClass schedulingv1.PriorityClass
			for _, prioClass := range priorityClassNames {
				key := types.NamespacedName{
					Name:      prioClass,
					Namespace: kitkyma.SystemNamespaceName,
				}
				err := K8sClient.Get(Ctx, key, &priorityClass)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
})
