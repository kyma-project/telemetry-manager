//go:build e2e

package traces

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

var _ = Describe(ID(), Label(LabelTraces), func() {
	var pipelineName = ID()
	const ownerReferenceKind = "TracePipeline"

	Context("When a TracePipeline exists", Ordered, func() {
		endpointKey := "traces-endpoint"
		secret := kitk8s.NewOpaqueSecret("traces-resources", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		tracePipeline := testutils.NewTracePipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			Build()

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, &tracePipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &tracePipeline, secret.K8sObject())).Should(Succeed())
		})

		It("Should have a ServiceAccount owned by the TracePipeline", func() {
			var serviceAccount corev1.ServiceAccount
			assert.HasOwnerReference(Ctx, K8sClient, &serviceAccount, kitkyma.TraceGatewayServiceAccount, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRole owned by the TracePipeline", func() {
			var clusterRole rbacv1.ClusterRole
			assert.HasOwnerReference(Ctx, K8sClient, &clusterRole, kitkyma.TraceGatewayClusterRole, ownerReferenceKind, pipelineName)
		})

		It("Should have a ClusterRoleBinding owned by the TracePipeline", func() {
			var clusterRoleBinding rbacv1.ClusterRoleBinding
			assert.HasOwnerReference(Ctx, K8sClient, &clusterRoleBinding, kitkyma.TraceGatewayClusterRoleBinding, ownerReferenceKind, pipelineName)
		})

		It("Should have a Metrics service owned by the TracePipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.TraceGatewayMetricsService, ownerReferenceKind, pipelineName)
		})

		It("Should have a Network Policy owned by the TracePipeline", func() {
			var networkPolicy networkingv1.NetworkPolicy
			assert.HasOwnerReference(Ctx, K8sClient, &networkPolicy, kitkyma.TraceGatewayNetworkPolicy, ownerReferenceKind, pipelineName)
		})

		It("Should have a Secret owned by the TracePipeline", func() {
			var secret corev1.Secret
			assert.HasOwnerReference(Ctx, K8sClient, &secret, kitkyma.TraceGatewaySecretName, ownerReferenceKind, pipelineName)
		})

		It("Should have a ConfigMap owned by the TracePipeline", func() {
			var configMap corev1.ConfigMap
			assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.TraceGatewayConfigMap, ownerReferenceKind, pipelineName)
		})

		It("Should have a Deployment owned by the TracePipeline", func() {
			var deployment appsv1.Deployment
			assert.HasOwnerReference(Ctx, K8sClient, &deployment, kitkyma.TraceGatewayName, ownerReferenceKind, pipelineName)
		})

		It("Should have an OTLP Service owned by the TracePipeline", func() {
			var service corev1.Service
			assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.TraceGatewayOTLPService, ownerReferenceKind, pipelineName)
		})

		It("Should have a Deployment with correct pod priority class", func() {
			assert.DeploymentHasPriorityClass(Ctx, K8sClient, kitkyma.TraceGatewayName, "telemetry-priority-class")
		})

		It("Should clean up gateway resources when pipeline becomes non-reconcilable", func() {
			By("Deleting referenced secret", func() {
				Expect(K8sClient.Delete(Ctx, secret.K8sObject())).Should(Succeed())
			})

			Eventually(func(g Gomega) bool {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayClusterRoleBinding, &clusterRoleBinding)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRoleBinding still exists")

			Eventually(func(g Gomega) bool {
				var serviceAccount corev1.ServiceAccount
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayServiceAccount, &serviceAccount)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ServiceAccount still exists")

			Eventually(func(g Gomega) bool {
				var clusterRole rbacv1.ClusterRole
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayClusterRole, &clusterRole)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRole still exists")

			Eventually(func(g Gomega) bool {
				var service corev1.Service
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayMetricsService, &service)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Metrics service still exists")

			Eventually(func(g Gomega) bool {
				var networkPolicy networkingv1.NetworkPolicy
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayNetworkPolicy, &networkPolicy)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "NetworkPolicy still exists")

			Eventually(func(g Gomega) bool {
				var secret corev1.Secret
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewaySecretName, &secret)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "TraceGatewaySecret still exists")

			Eventually(func(g Gomega) bool {
				var configMap corev1.ConfigMap
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayConfigMap, &configMap)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "TraceGatewayConfigMap still exists")

			Eventually(func(g Gomega) bool {
				var deployment appsv1.Deployment
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayName, &deployment)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Deployment still exists")

			Eventually(func(g Gomega) bool {
				var service corev1.Service
				err := K8sClient.Get(Ctx, kitkyma.TraceGatewayOTLPService, &service)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "OTLP service still exists")
		})
	})
})
