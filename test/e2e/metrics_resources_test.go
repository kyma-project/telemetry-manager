//go:build e2e

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/kyma-project/telemetry-manager/internal/testutils"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

var _ = Describe(suite.ID(), Label(suite.LabelMetrics), Ordered, func() {
	var pipelineName = suite.ID()
	const ownerReferenceKind = "MetricPipeline"

	Context("When a MetricPipeline exists", Ordered, func() {
		endpointKey := "metrics-endpoint"
		secret := kitk8s.NewOpaqueSecret("metrics-resources", kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, "http://localhost:4317"))
		metricPipeline := testutils.NewMetricPipelineBuilder().
			WithName(pipelineName).
			WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
			WithRuntimeInput(true).
			Build()

		BeforeAll(func() {
			DeferCleanup(func() {
				Expect(kitk8s.DeleteObjects(ctx, k8sClient, &metricPipeline)).Should(Succeed())
			})
			Expect(kitk8s.CreateObjects(ctx, k8sClient, &metricPipeline, secret.K8sObject())).Should(Succeed())
		})

		Context("Should have gateway resources", Ordered, func() {
			It("Should have a gateway ServiceAccount owned by the MetricPipeline", func() {
				var serviceAccount corev1.ServiceAccount
				assert.HasOwnerReference(ctx, k8sClient, &serviceAccount, kitkyma.MetricGatewayServiceAccount, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway ClusterRole owned by the MetricPipeline", func() {
				var clusterRole rbacv1.ClusterRole
				assert.HasOwnerReference(ctx, k8sClient, &clusterRole, kitkyma.MetricGatewayClusterRole, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway ClusterRoleBinding owned by the MetricPipeline", func() {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				assert.HasOwnerReference(ctx, k8sClient, &clusterRoleBinding, kitkyma.MetricGatewayClusterRoleBinding, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Metrics service owned by the MetricPipeline", func() {
				var service corev1.Service
				assert.HasOwnerReference(ctx, k8sClient, &service, kitkyma.MetricGatewayMetricsService, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Network Policy owned by the MetricPipeline", func() {
				var networkPolicy networkingv1.NetworkPolicy
				assert.HasOwnerReference(ctx, k8sClient, &networkPolicy, kitkyma.MetricGatewayNetworkPolicy, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Secret owned by the MetricPipeline", func() {
				var secret corev1.Secret
				assert.HasOwnerReference(ctx, k8sClient, &secret, kitkyma.MetricGatewaySecretName, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway ConfigMap owned by the MetricPipeline", func() {
				var configMap corev1.ConfigMap
				assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.MetricGatewayConfigMap, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Deployment owned by the MetricPipeline", func() {
				var deployment appsv1.Deployment
				assert.HasOwnerReference(ctx, k8sClient, &deployment, kitkyma.MetricGatewayName, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway OTLP Service owned by the MetricPipeline", func() {
				var service corev1.Service
				assert.HasOwnerReference(ctx, k8sClient, &service, kitkyma.MetricGatewayOTLPService, ownerReferenceKind, pipelineName)
			})

			It("Should have a gateway Deployment with correct pod priority class", func() {
				assert.DeploymentHasPriorityClass(ctx, k8sClient, kitkyma.MetricGatewayName, "telemetry-priority-class")
			})
		})

		Context("Should have agent resources", Ordered, func() {
			It("Should have an agent ServiceAccount owned by the MetricPipeline", func() {
				var serviceAccount corev1.ServiceAccount
				assert.HasOwnerReference(ctx, k8sClient, &serviceAccount, kitkyma.MetricAgentServiceAccount, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent ClusterRole owned by the MetricPipeline", func() {
				var clusterRole rbacv1.ClusterRole
				assert.HasOwnerReference(ctx, k8sClient, &clusterRole, kitkyma.MetricAgentClusterRole, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent ClusterRoleBinding owned by the MetricPipeline", func() {
				var clusterRoleBinding rbacv1.ClusterRoleBinding
				assert.HasOwnerReference(ctx, k8sClient, &clusterRoleBinding, kitkyma.MetricAgentClusterRoleBinding, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent Metrics service owned by the MetricPipeline", func() {
				var service corev1.Service
				assert.HasOwnerReference(ctx, k8sClient, &service, kitkyma.MetricAgentMetricsService, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent Network Policy owned by the MetricPipeline", func() {
				var networkPolicy networkingv1.NetworkPolicy
				assert.HasOwnerReference(ctx, k8sClient, &networkPolicy, kitkyma.MetricAgentNetworkPolicy, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent ConfigMap owned by the MetricPipeline", func() {
				var configMap corev1.ConfigMap
				assert.HasOwnerReference(ctx, k8sClient, &configMap, kitkyma.MetricAgentConfigMap, ownerReferenceKind, pipelineName)
			})

			It("Should have an agent DaemonSet owned by the MetricPipeline", func() {
				var daemonSet appsv1.DaemonSet
				assert.HasOwnerReference(ctx, k8sClient, &daemonSet, kitkyma.MetricAgentName, ownerReferenceKind, pipelineName)
			})
		})

		It("Should clean up gateway and agent resources when pipeline becomes non-reconcilable", func() {
			By("Deleting referenced secret", func() {
				Expect(k8sClient.Delete(ctx, secret.K8sObject())).Should(Succeed())
			})
			gatewayResourcesAreDeleted()
			agentResourcesAreDeleted()
		})

	})
})

func gatewayResourcesAreDeleted() {
	Eventually(func(g Gomega) bool {
		var serviceAccount corev1.ServiceAccount
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayServiceAccount, &serviceAccount)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ServiceAccount not deleted"))

	Eventually(func(g Gomega) bool {
		var clusterRole rbacv1.ClusterRole
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayClusterRole, &clusterRole)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ClusterRole not deleted"))

	Eventually(func(g Gomega) bool {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayClusterRoleBinding, &clusterRoleBinding)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ClusterRoleBinding not deleted"))

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayMetricsService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("Service not deleted"))

	Eventually(func(g Gomega) bool {
		var networkPolicy networkingv1.NetworkPolicy
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayNetworkPolicy, &networkPolicy)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("NetworkPolicy not deleted"))

	Eventually(func(g Gomega) bool {
		var secret corev1.Secret
		err := k8sClient.Get(ctx, kitkyma.MetricGatewaySecretName, &secret)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("Secret not deleted"))

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ConfigMap not deleted"))

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayOTLPService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("OTLP Service not deleted"))

	Eventually(func(g Gomega) bool {
		var deployment appsv1.Deployment
		err := k8sClient.Get(ctx, kitkyma.MetricGatewayName, &deployment)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("Deployment not deleted"))
}

func agentResourcesAreDeleted() {
	Eventually(func(g Gomega) bool {
		var serviceAccount corev1.ServiceAccount
		err := k8sClient.Get(ctx, kitkyma.MetricAgentServiceAccount, &serviceAccount)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ServiceAccount not deleted"))

	Eventually(func(g Gomega) bool {
		var clusterRole rbacv1.ClusterRole
		err := k8sClient.Get(ctx, kitkyma.MetricAgentClusterRole, &clusterRole)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ClusterRole not deleted"))

	Eventually(func(g Gomega) bool {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := k8sClient.Get(ctx, kitkyma.MetricAgentClusterRoleBinding, &clusterRoleBinding)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ClusterRoleBinding not deleted"))

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := k8sClient.Get(ctx, kitkyma.MetricAgentMetricsService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("Service not deleted"))

	Eventually(func(g Gomega) bool {
		var networkPolicy networkingv1.NetworkPolicy
		err := k8sClient.Get(ctx, kitkyma.MetricAgentNetworkPolicy, &networkPolicy)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("NetworkPolicy not deleted"))

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := k8sClient.Get(ctx, kitkyma.MetricAgentConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("ConfigMap not deleted"))

	Eventually(func(g Gomega) bool {
		var daemonSet appsv1.DaemonSet
		err := k8sClient.Get(ctx, kitkyma.MetricAgentName, &daemonSet)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrueBecause("DaemonSet not deleted"))
}
