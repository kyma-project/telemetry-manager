//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

var _ = Describe(suite.ID(), Label(suite.LabelLogs), Ordered, func() {
	var pipelineName = suite.ID()
	const ownerReferenceKind = "LogPipeline"

	Context("When a LogPipeline exists", Ordered, func() {

		BeforeAll(func() {
			pipeline := testutils.NewLogPipelineBuilder().WithName(pipelineName).Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &pipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &pipeline)).Should(Succeed())
		})

		It("Should have a ServiceAccount owned by the LogPipeline", func() {
			var serviceAccount corev1.ServiceAccount
			assert.HasOwnerReference(ctx, k8sClient, &serviceAccount, kitkyma.FluentBitServiceAccount, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRole owned by the LogPipeline", func() {
			var clusterRole rbacv1.ClusterRole
			assert.HasOwnerReference(ctx, k8sClient, &clusterRole, kitkyma.FluentBitClusterRole, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRoleBinding owned by the LogPipeline", func() {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			assert.HasOwnerReference(ctx, k8sClient, &clusterRoleBinding, kitkyma.FluentBitClusterRoleBinding, ownerReferenceKind, pipelineName)
		})

		It("Should have an exporter metrics Service owned by the LogPipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(ctx, k8sClient, &service, kitkyma.FluentBitExporterMetricsService, ownerReferenceKind, pipelineName)
		})

		It("Should have a metrics Service owned by the LogPipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(ctx, k8sClient, &service, kitkyma.FluentBitMetricsService, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.FluentBitConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit-luascripts ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.FluentBitLuaConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit-parsers ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.FluentBiParserConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a telemetry-fluent-bit-files ConfigMap owned by the LogPipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.FluentBiFilesConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a DaemonSet owned by the LogPipeline", func() {
			var daemonSet appsv1.DaemonSet
			assert.HasOwnerReference(ctx, k8sClient, &daemonSet, kitkyma.FluentBitDaemonSet, ownerReferenceKind, pipelineName)
		})

		It("Should have a Network Policy owned by the LogPipeline", func() {
			var networkPolicy networkingv1.NetworkPolicy
			assert.HasOwnerReference(ctx, k8sClient, &networkPolicy, kitkyma.FluentBiNetworkPolicy, ownerReferenceKind, pipelineName)
		})

		It("Should have a DaemonSet with correct pod priority class", func() {
			Eventually(func(g Gomega) {
				var daemonSet appsv1.DaemonSet
				g.Expect(k8sClient.Get(ctx, kitkyma.FluentBitDaemonSet, &daemonSet)).To(Succeed())

				g.Expect(daemonSet.Spec.Template.Spec.PriorityClassName).To(Equal("telemetry-priority-class-high"))
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

	})
})
