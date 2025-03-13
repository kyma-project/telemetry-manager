//go:build e2e

package fluentbit

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	. "github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(ID(), Label(LabelLogs), Ordered, func() {
	var pipelineName = ID()
	const ownerReferenceKind = "LogPipeline"

	Context("When a LogPipeline exists", Ordered, func() {
		endpointKey := "logs-endpoint"
		secret := kitk8s.NewOpaqueSecret("logs-resources", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:123"))
		pipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).WithHTTPOutput(testutils.HTTPHostFromSecret(secret.Name(), kitkyma.DefaultNamespaceName, endpointKey)).Build()

		BeforeAll(func() {

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, &pipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &pipeline, secret.K8sObject())).Should(Succeed())
		})

		It("Should have a ServiceAccount owned by the LogPipeline", func() {
			var serviceAccount corev1.ServiceAccount
			assert.HasOwnerReference(Ctx, K8sClient, &serviceAccount, kitkyma.FluentBitServiceAccount, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRole owned by the LogPipeline", func() {
			var clusterRole rbacv1.ClusterRole
			assert.HasOwnerReference(Ctx, K8sClient, &clusterRole, kitkyma.FluentBitClusterRole, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRoleBinding owned by the LogPipeline", func() {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			assert.HasOwnerReference(Ctx, K8sClient, &clusterRoleBinding, kitkyma.FluentBitClusterRoleBinding, ownerReferenceKind, pipelineName)
		})

		It("Should have an exporter metrics Service owned by the LogPipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.FluentBitExporterMetricsService, ownerReferenceKind, pipelineName)
		})

		It("Should have a metrics Service owned by the LogPipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.FluentBitMetricsService, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.FluentBitConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit-luascripts ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.FluentBitLuaConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit-parsers ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.FluentBitParserConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit-files ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.FluentBitFilesConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a DaemonSet owned by the LogPipeline", func() {
			var daemonSet appsv1.DaemonSet
			assert.HasOwnerReference(Ctx, K8sClient, &daemonSet, kitkyma.FluentBitDaemonSetName, ownerReferenceKind, pipelineName)
		})

		It("Should have a Network Policy owned by the LogPipeline", func() {
			var networkPolicy networkingv1.NetworkPolicy
			assert.HasOwnerReference(Ctx, K8sClient, &networkPolicy, kitkyma.FluentBitNetworkPolicy, ownerReferenceKind, pipelineName)
		})

		It("Should have a DaemonSet with correct pod priority class", func() {
			Eventually(func(g Gomega) {
				var daemonSet appsv1.DaemonSet
				g.Expect(K8sClient.Get(Ctx, kitkyma.FluentBitDaemonSetName, &daemonSet)).To(Succeed())

				g.Expect(daemonSet.Spec.Template.Spec.PriorityClassName).To(Equal("telemetry-priority-class-high"))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})
		It("Should clean up log pipeline resources when pipeline becomes non-reconcilable", func() {
			By("Deleting referenced secret", func() {
				Expect(K8sClient.Delete(Ctx, secret.K8sObject())).Should(Succeed())
			})

			Eventually(func(g Gomega) bool {
				var serviceAccount corev1.ServiceAccount
				err := K8sClient.Get(Ctx, kitkyma.FluentBitServiceAccount, &serviceAccount)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ServiceAccount still exists")

			Eventually(func(g Gomega) bool {
				var clusterRole rbacv1.ClusterRole
				err := K8sClient.Get(Ctx, kitkyma.FluentBitClusterRole, &clusterRole)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRole still exists")

			Eventually(func(g Gomega) bool {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				err := K8sClient.Get(Ctx, kitkyma.FluentBitClusterRoleBinding, &clusterRoleBinding)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRoleBinding still exists")

			Eventually(func(g Gomega) bool {
				var service corev1.Service
				err := K8sClient.Get(Ctx, kitkyma.FluentBitExporterMetricsService, &service)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Exporter metrics service still exists")

			Eventually(func(g Gomega) bool {
				var service corev1.Service
				err := K8sClient.Get(Ctx, kitkyma.FluentBitMetricsService, &service)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Metrics service still exists")

			Eventually(func(g Gomega) bool {
				var networkPolicy networkingv1.NetworkPolicy
				err := K8sClient.Get(Ctx, kitkyma.FluentBitNetworkPolicy, &networkPolicy)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Network policy still exists")

			Eventually(func(g Gomega) bool {
				var configMap corev1.ConfigMap
				err := K8sClient.Get(Ctx, kitkyma.FluentBitConfigMap, &configMap)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ConfigMap still exists")

			Eventually(func(g Gomega) bool {
				var configMap corev1.ConfigMap
				err := K8sClient.Get(Ctx, kitkyma.FluentBitLuaConfigMap, &configMap)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Lua ConfigMap still exists")

			Eventually(func(g Gomega) bool {
				var configMap corev1.ConfigMap
				err := K8sClient.Get(Ctx, kitkyma.FluentBitParserConfigMap, &configMap)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Parser ConfigMap still exists")

			Eventually(func(g Gomega) bool {
				var configMap corev1.ConfigMap
				err := K8sClient.Get(Ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Sections ConfigMap still exists")

			Eventually(func(g Gomega) bool {
				var configMap corev1.ConfigMap
				err := K8sClient.Get(Ctx, kitkyma.FluentBitFilesConfigMap, &configMap)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Files ConfigMap still exists")

			Eventually(func(g Gomega) bool {
				var daemonSet appsv1.DaemonSet
				err := K8sClient.Get(Ctx, kitkyma.FluentBitDaemonSetName, &daemonSet)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "DaemonSet still exists")

			Eventually(func(g Gomega) bool {
				var service corev1.Service
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayOTLPService, &service)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "OTLP service still exists")
		})
	})
})
