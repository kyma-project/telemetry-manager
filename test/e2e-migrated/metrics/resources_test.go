package metrics

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestResources(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelMetricsSetC)

	const (
		endpointKey   = "metrics-endpoint"
		endpointValue = "http://localhost:4317"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()

		gatewayResources = []assert.Resource{
			assert.NewResource(&appsv1.Deployment{}, kitkyma.MetricGatewayName),
			assert.NewResource(&corev1.Service{}, kitkyma.MetricGatewayMetricsService),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.MetricGatewayServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.MetricGatewayClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.MetricGatewayClusterRoleBinding),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.MetricGatewayNetworkPolicy),
			assert.NewResource(&corev1.Secret{}, kitkyma.MetricGatewaySecretName),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.MetricGatewayConfigMap),
			assert.NewResource(&corev1.Service{}, kitkyma.MetricGatewayOTLPService),
			assert.NewResource(&rbacv1.Role{}, kitkyma.MetricGatewayRole),
			assert.NewResource(&rbacv1.RoleBinding{}, kitkyma.MetricGatewayRoleBinding),
		}

		agentResources = []assert.Resource{
			assert.NewResource(&appsv1.DaemonSet{}, kitkyma.MetricAgentName),
			assert.NewResource(&corev1.Service{}, kitkyma.MetricAgentMetricsService),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.MetricAgentServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.MetricAgentClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.MetricAgentClusterRoleBinding),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.MetricAgentNetworkPolicy),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.MetricAgentConfigMap),
		}
	)

	secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, endpointValue))
	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		WithRuntimeInput(true).
		Build()

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(&pipeline))
	})
	Expect(kitk8s.CreateObjects(t, &pipeline, secret.K8sObject())).Should(Succeed())

	assert.ResourcesExist(t, gatewayResources...)
	assert.ResourcesExist(t, agentResources...)

	t.Log("When MetricPipeline becomes non-reconcilable, resources should be cleaned up")
	Expect(suite.K8sClient.Delete(t.Context(), secret.K8sObject())).Should(Succeed())
	assert.ResourcesNotExist(t, gatewayResources...)
	assert.ResourcesNotExist(t, agentResources...)
}
