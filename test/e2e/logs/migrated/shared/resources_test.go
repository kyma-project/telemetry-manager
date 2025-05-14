package shared

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

// TODO: TO BE FIXED
func TestResources_OTel(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name                     string
		input                    telemetryv1alpha1.LogPipelineInput
		mainResourceToCheckFor   assert.Resource
		otherResourcesToCheckFor []assert.Resource
	}{
		{
			name:                   "agent",
			input:                  testutils.BuildLogPipelineApplicationInput(),
			mainResourceToCheckFor: assert.NewResource[*appsv1.DaemonSet](kitkyma.LogAgentName),
			otherResourcesToCheckFor: []assert.Resource{
				assert.NewResource[*corev1.ServiceAccount](kitkyma.LogAgentServiceAccount),
				assert.NewResource[*rbacv1.ClusterRole](kitkyma.LogAgentClusterRole),
				assert.NewResource[*rbacv1.ClusterRoleBinding](kitkyma.LogAgentClusterRoleBinding),
				assert.NewResource[*corev1.Service](kitkyma.LogAgentMetricsService),
				assert.NewResource[*networkingv1.NetworkPolicy](kitkyma.LogAgentNetworkPolicy),
				assert.NewResource[*corev1.ConfigMap](kitkyma.LogAgentConfigMap),
			},
		},
		{
			name:                   "gateway",
			input:                  testutils.BuildLogPipelineOTLPInput(),
			mainResourceToCheckFor: assert.NewResource[*appsv1.Deployment](kitkyma.LogGatewayName),
			otherResourcesToCheckFor: []assert.Resource{
				assert.NewResource[*corev1.ServiceAccount](kitkyma.LogGatewayServiceAccount),
				assert.NewResource[*rbacv1.ClusterRole](kitkyma.LogGatewayClusterRole),
				assert.NewResource[*rbacv1.ClusterRoleBinding](kitkyma.LogGatewayClusterRoleBinding),
				assert.NewResource[*corev1.Service](kitkyma.LogGatewayMetricsService),
				assert.NewResource[*networkingv1.NetworkPolicy](kitkyma.LogGatewayNetworkPolicy),
				assert.NewResource[*corev1.ConfigMap](kitkyma.LogGatewayConfigMap),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			const (
				endpointKey = "endpoint"
				endpoint    = "http://localhost:123"
			)

			var (
				uniquePrefix = unique.Prefix()
				pipelineName = uniquePrefix()
				secretName   = uniquePrefix()
			)

			secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, endpoint))
			pipeline := testutils.NewLogPipelineBuilder().
				WithInput(tc.input).
				WithName(pipelineName).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), kitkyma.DefaultNamespaceName, endpointKey)).
				Build()

			t.Cleanup(func() {
				require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &pipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
			})
			Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, &pipeline, secret.K8sObject())).Should(Succeed())

			assert.ResourcesExist(t.Context(), suite.K8sClient, tc.mainResourceToCheckFor)

			Expect(suite.K8sClient.Delete(suite.Ctx, secret.K8sObject())).Should(Succeed())

			assert.ResourcesExist(t.Context(), suite.K8sClient, tc.otherResourcesToCheckFor...)
			assert.ResourcesExist(t.Context(), suite.K8sClient, tc.mainResourceToCheckFor) // Main resource still exists

			Eventually(func(g Gomega) bool {
				var service corev1.Service
				err := suite.K8sClient.Get(suite.Ctx, kitkyma.LogGatewayOTLPService, &service)
				return apierrors.IsNotFound(err)
			}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "OTLP service still exists")
		})
	}
}

func TestResources_FluentBit(t *testing.T) {
	RegisterTestingT(t)

	const (
		endpointKey = "endpoint"
		endpoint    = "http://localhost:123"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()
	)

	secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, endpoint))
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHostFromSecret(secret.Name(), kitkyma.DefaultNamespaceName, endpointKey)).
		Build()

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &pipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, &pipeline, secret.K8sObject())).Should(Succeed())

	Eventually(func(g Gomega) {
		var daemonSet appsv1.DaemonSet
		g.Expect(suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitDaemonSetName, &daemonSet)).To(Succeed())

		g.Expect(daemonSet.Spec.Template.Spec.PriorityClassName).To(Equal("telemetry-priority-class-high"))
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())

	Expect(suite.K8sClient.Delete(suite.Ctx, secret.K8sObject())).Should(Succeed())

	Eventually(func(g Gomega) bool {
		var serviceAccount corev1.ServiceAccount
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitServiceAccount, &serviceAccount)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ServiceAccount still exists")

	Eventually(func(g Gomega) bool {
		var clusterRole rbacv1.ClusterRole
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitClusterRole, &clusterRole)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRole still exists")

	Eventually(func(g Gomega) bool {
		var clusterRoleBinding rbacv1.ClusterRoleBinding
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitClusterRoleBinding, &clusterRoleBinding)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ClusterRoleBinding still exists")

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitExporterMetricsService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Exporter metrics service still exists")

	Eventually(func(g Gomega) bool {
		var service corev1.Service
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitMetricsService, &service)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Metrics service still exists")

	Eventually(func(g Gomega) bool {
		var networkPolicy networkingv1.NetworkPolicy
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitNetworkPolicy, &networkPolicy)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Network policy still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitLuaConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Lua ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitParserConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Parser ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitSectionsConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Sections ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var configMap corev1.ConfigMap
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitFilesConfigMap, &configMap)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "Files ConfigMap still exists")

	Eventually(func(g Gomega) bool {
		var daemonSet appsv1.DaemonSet
		err := suite.K8sClient.Get(suite.Ctx, kitkyma.FluentBitDaemonSetName, &daemonSet)
		return apierrors.IsNotFound(err)
	}, periodic.EventuallyTimeout, periodic.DefaultInterval).Should(BeTrue(), "DaemonSet still exists")
}
