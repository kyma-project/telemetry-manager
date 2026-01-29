package istio

import (
	"testing"

	. "github.com/onsi/gomega"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestMetricsResources(t *testing.T) {
	// This test need to run with istio installed in the cluster to be able to test the creation and reconciliation of PeerAuthentication
	suite.RegisterTestCase(t, suite.LabelIstio)

	const (
		endpointKey   = "metrics-endpoint"
		endpointValue = "http://localhost:4317"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()
		resources    = []assert.Resource{
			// Agent resources
			assert.NewResource(&appsv1.DaemonSet{}, kitkyma.MetricAgentName),
			assert.NewResource(&corev1.Service{}, kitkyma.MetricAgentMetricsService),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.MetricAgentServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.MetricAgentClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.MetricAgentClusterRoleBinding),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.MetricAgentNetworkPolicy),
			assert.NewResource(&corev1.Secret{}, kitkyma.MetricAgentSecretName),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.MetricAgentConfigMap),

			// Gateway resources
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
			assert.NewResource(&istiosecurityclientv1.PeerAuthentication{}, kitkyma.MetricGatewayPeerAuthentication),
		}
	)

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, endpointValue))
	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		WithOTLPInput(true).
		WithRuntimeInput(true).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline, secret.K8sObject())).To(Succeed())

	assert.ResourcesExist(t, resources...)

	assert.ResourcesReconciled(t, resources...)

	// FIXME: Flaky behavior, colliding with resources from other tests
	// t.Log("When MetricPipeline becomes non-reconcilable, resources should be cleaned up")
	// Expect(suite.K8sClient.Delete(t.Context(), secret.K8sObject())).To(Succeed())
	// assert.ResourcesNotExist(t, tc.resources...)
}
