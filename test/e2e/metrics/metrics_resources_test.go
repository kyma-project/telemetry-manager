//go:build e2e

package metrics

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

var _ = Describe(ID(), Ordered, func() {
	const ownerReferenceKind = "MetricPipeline"

	Context("When a MetricPipeline exists", Label(LabelMetrics), Label(LabelSetC), Ordered, func() {
		var pipelineName = ID()
		endpointKey := "metrics-endpoint"
		secret := kitk8s.NewOpaqueSecret("metrics-resources", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			WithRuntimeInput(true).
			Build()

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, &metricPipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &metricPipeline, secret.K8sObject())).Should(Succeed())
		})

		Context("Should have gateway resources", Ordered, func() {
			It("Should have a gateway ServiceAccount owned by the MetricPipeline", func() {
				var serviceAccount corev1.ServiceAccount
				assert.HasOwnerReference(Ctx, K8sClient, &serviceAccount, kitkyma.MetricGatewayServiceAccount, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway ClusterRole owned by the MetricPipeline", func() {
				var clusterRole rbacv1.ClusterRole
				assert.HasOwnerReference(Ctx, K8sClient, &clusterRole, kitkyma.MetricGatewayClusterRole, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway ClusterRoleBinding owned by the MetricPipeline", func() {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				assert.HasOwnerReference(Ctx, K8sClient, &clusterRoleBinding, kitkyma.MetricGatewayClusterRoleBinding, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Metrics service owned by the MetricPipeline", func() {
				var service corev1.Service
				assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.MetricGatewayMetricsService, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Network Policy owned by the MetricPipeline", func() {
				var networkPolicy networkingv1.NetworkPolicy
				assert.HasOwnerReference(Ctx, K8sClient, &networkPolicy, kitkyma.MetricGatewayNetworkPolicy, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Secret owned by the MetricPipeline", func() {
				var secret corev1.Secret
				assert.HasOwnerReference(Ctx, K8sClient, &secret, kitkyma.MetricGatewaySecretName, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway ConfigMap owned by the MetricPipeline", func() {
				var configMap corev1.ConfigMap
				assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.MetricGatewayConfigMap, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Deployment owned by the MetricPipeline", func() {
				var deployment appsv1.Deployment
				assert.HasOwnerReference(Ctx, K8sClient, &deployment, kitkyma.MetricGatewayName, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway OTLP Service owned by the MetricPipeline", func() {
				var service corev1.Service
				assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.MetricGatewayOTLPService, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Deployment with correct pod priority class", func() {
				assert.DeploymentHasPriorityClass(Ctx, K8sClient, kitkyma.MetricGatewayName, "telemetry-priority-class")
			})
		})

		Context("Should have agent resources", Ordered, func() {
			It("Should have an agent ServiceAccount owned by the MetricPipeline", func() {
				var serviceAccount corev1.ServiceAccount
				assert.HasOwnerReference(Ctx, K8sClient, &serviceAccount, kitkyma.MetricAgentServiceAccount, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent ClusterRole owned by the MetricPipeline", func() {
				var clusterRole rbacv1.ClusterRole
				assert.HasOwnerReference(Ctx, K8sClient, &clusterRole, kitkyma.MetricAgentClusterRole, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent ClusterRoleBinding owned by the MetricPipeline", func() {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				assert.HasOwnerReference(Ctx, K8sClient, &clusterRoleBinding, kitkyma.MetricAgentClusterRoleBinding, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent Metrics service owned by the MetricPipeline", func() {
				var service corev1.Service
				assert.HasOwnerReference(Ctx, K8sClient, &service, kitkyma.MetricAgentMetricsService, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent Network Policy owned by the MetricPipeline", func() {
				var networkPolicy networkingv1.NetworkPolicy
				assert.HasOwnerReference(Ctx, K8sClient, &networkPolicy, kitkyma.MetricAgentNetworkPolicy, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent ConfigMap owned by the MetricPipeline", func() {
				var configMap corev1.ConfigMap
				assert.HasOwnerReference(Ctx, K8sClient, &configMap, kitkyma.MetricAgentConfigMap, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent DaemonSet owned by the MetricPipeline", func() {
				var daemonSet appsv1.DaemonSet
				assert.HasOwnerReference(Ctx, K8sClient, &daemonSet, kitkyma.MetricAgentName, ownerReferenceKind, pipelineName)
			})
		})

		It("Should clean up gateway and agent resources when pipeline becomes non-reconcilable", func() {
			By("Deleting referenced secret", func() {
				Expect(K8sClient.Delete(Ctx, secret.K8sObject())).Should(Succeed())
			})
			gatewayResourcesAreDeleted()
			agentResourcesAreDeleted()
		})

	})

	// TODO: Move the tests in this Context to the Context above ("When a MetricPipeline exists") when the feature flag --kyma-input-allowed is removed
	Context("When a MetricPipeline exists in experimental channel", Label(LabelMetrics, LabelExperimental), Ordered, func() {
		var pipelineName = IDWithSuffix("experimental")
		endpointKey := "metrics-endpoint"
		secret := kitk8s.NewOpaqueSecret("metrics-resources", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			WithRuntimeInput(true).
			Build()

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(Ctx, K8sClient, &metricPipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(Ctx, K8sClient, &metricPipeline, secret.K8sObject())).Should(Succeed())
		})

		Context("should have experimental gateway resources", Ordered, func() {
			It("Should have a gateway Role owned by the MetricPipeline", func() {
				var role rbacv1.Role
				assert.HasOwnerReference(Ctx, K8sClient, &role, kitkyma.MetricGatewayRole, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway RoleBinding owned by the MetricPipeline", func() {
				var roleBinding rbacv1.RoleBinding
				assert.HasOwnerReference(Ctx, K8sClient, &roleBinding, kitkyma.MetricGatewayRoleBinding, ownerReferenceKind, pipelineName)
			})
		})

		It("Should clean up experimental gateway resources when pipeline becomes non-reconcilable", func() {
			By("Deleting referenced secret", func() {
				Expect(K8sClient.Delete(Ctx, secret.K8sObject())).Should(Succeed())
			})

			Eventually(func(g Gomega) bool {
				var role rbacv1.Role
				err := K8sClient.Get(Ctx, kitkyma.MetricGatewayRole, &role)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Role still exists")

			Eventually(func(g Gomega) bool {
				var roleBinding rbacv1.ClusterRoleBinding
				err := K8sClient.Get(Ctx, kitkyma.MetricGatewayRoleBinding, &roleBinding)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "RoleBinding still exists")

		})

	})
})

func gatewayResourcesAreDeleted() {
	Eventually(func(g Gomega) bool {
		var serviceAccount corev1.ServiceAccount
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayServiceAccount, &serviceAccount)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ServiceAccount still exists")

	Eventually(func(g Gomega) bool {
		var clusterRole rbacv1.ClusterRole
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayClusterRole, &clusterRole)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRole still exists")

	Eventually(func(g Gomega) bool {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayClusterRoleBinding, &clusterRoleBinding)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRoleBinding still exists")

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayMetricsService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Service still exists")

	Eventually(func(g Gomega) bool {
		var networkPolicy networkingv1.NetworkPolicy
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayNetworkPolicy, &networkPolicy)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "NetworkPolicy still exists")

	Eventually(func(g Gomega) bool {
		var secret corev1.Secret
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewaySecretName, &secret)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Secret still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayOTLPService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "OTLP Service still exists")

	Eventually(func(g Gomega) bool {
		var deployment appsv1.Deployment
		err := K8sClient.Get(Ctx, kitkyma.MetricGatewayName, &deployment)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Deployment still exists")
}

func agentResourcesAreDeleted() {
	Eventually(func(g Gomega) bool {
		var serviceAccount corev1.ServiceAccount
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentServiceAccount, &serviceAccount)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ServiceAccount still exists")

	Eventually(func(g Gomega) bool {
		var clusterRole rbacv1.ClusterRole
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentClusterRole, &clusterRole)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRole still exists")

	Eventually(func(g Gomega) bool {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentClusterRoleBinding, &clusterRoleBinding)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRoleBinding still exists")

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentMetricsService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Service still exists")

	Eventually(func(g Gomega) bool {
		var networkPolicy networkingv1.NetworkPolicy
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentNetworkPolicy, &networkPolicy)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "NetworkPolicy still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var daemonSet appsv1.DaemonSet
		err := K8sClient.Get(Ctx, kitkyma.MetricAgentName, &daemonSet)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "DaemonSet still exists")
}
