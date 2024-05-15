//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelTraces), func() {
	var pipelineName = suite.ID()
	const ownerReferenceKind = "TracePipeline"

	Context("When a TracePipeline exists", Ordered, func() {

		BeforeAll(func() {
			tracePipeline := testutils.NewTracePipelineBuilder().
				WithName(pipelineName).
				Build()

			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &tracePipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &tracePipeline)).Should(Succeed())
		})

		It("Should have a ServiceAccount owned by the TracePipeline", func() {
			var serviceAccount corev1.ServiceAccount
			assert.HasOwnerReference(ctx, k8sClient, &serviceAccount, kitkyma.TraceGatewayServiceAccount, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRole owned by the TracePipeline", func() {
			var clusterRole rbacv1.ClusterRole
			assert.HasOwnerReference(ctx, k8sClient, &clusterRole, kitkyma.TraceGatewayClusterRole, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRoleBinding owned by the TracePipeline", func() {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			assert.HasOwnerReference(ctx, k8sClient, &clusterRoleBinding, kitkyma.TraceGatewayClusterRoleBinding, ownerReferenceKind, pipelineName)
		})

		It("Should have a Deployment owned by the TracePipeline", func() {
			var deployment appsv1.Deployment
			assert.HasOwnerReference(ctx, k8sClient, &deployment, kitkyma.TraceGatewayName, ownerReferenceKind, pipelineName)
		})

		It("Should have a Service owned by the TracePipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(ctx, k8sClient, &service, kitkyma.TraceGatewayService, ownerReferenceKind, pipelineName)
		})

		It("Should have a ConfigMap owned by the TracePipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.TraceGatewayConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a Secret owned by the TracePipeline", func() {
			var secret corev1.Secret
			assert.HasOwnerReference(ctx, k8sClient, &secret, kitkyma.TraceGatewaySecretName, ownerReferenceKind, pipelineName)
		})

		It("Should have a Deployment with correct pod environment configuration", func() {
			assert.DeploymentHasEnvFromSecret(ctx, k8sClient, kitkyma.TraceGatewayName, kitkyma.TraceGatewayBaseName)
		})

		It("Should have a Deployment with correct pod metadata", func() {
			Eventually(func(g Gomega) {
				var deployment appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, kitkyma.TraceGatewayName, &deployment)).To(Succeed())

				g.Expect(deployment.Spec.Template.ObjectMeta.Labels["sidecar.istio.io/inject"]).To(Equal("false"))
				g.Expect(deployment.Spec.Template.ObjectMeta.Annotations["checksum/config"]).ToNot(BeEmpty())
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
		})

		It("Should have a Deployment with correct pod priority class", func() {
			assert.DeploymentHasPriorityClass(ctx, k8sClient, kitkyma.TraceGatewayName, "telemetry-priority-class")
		})

	})
})
