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

func TestTracesResources(t *testing.T) {
	// This test need to run with istio installed in the cluster to be able to test the creation and reconciliation of PeerAuthentication
	suite.RegisterTestCase(t, suite.LabelIstio)

	const (
		endpointKey   = "traces-endpoint"
		endpointValue = "http://localhost:4317"
	)

	var (
		uniquePrefix     = unique.Prefix()
		pipelineName     = uniquePrefix()
		secretName       = uniquePrefix()
		gatewayResources = []assert.Resource{
			assert.NewResource(&appsv1.Deployment{}, kitkyma.TraceGatewayName),
			assert.NewResource(&corev1.Service{}, kitkyma.TraceGatewayMetricsService),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.TraceGatewayServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.TraceGatewayClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.TraceGatewayClusterRoleBinding),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.TraceGatewayNetworkPolicy),
			assert.NewResource(&corev1.Secret{}, kitkyma.TraceGatewaySecretName),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.TraceGatewayConfigMap),
			assert.NewResource(&corev1.Service{}, kitkyma.TraceGatewayOTLPService),
			assert.NewResource(&istiosecurityclientv1.PeerAuthentication{}, kitkyma.TraceGatewayPeerAuthentication),
		}
	)

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, endpointValue))
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline, secret.K8sObject())).To(Succeed())

	assert.ResourcesExist(t, gatewayResources...)

	assert.ResourcesReconciled(t, gatewayResources...)

	t.Log("When TracePipeline becomes non-reconcilable, resources should be cleaned up")
	Expect(suite.K8sClient.Delete(t.Context(), secret.K8sObject())).To(Succeed())
	assert.ResourcesNotExist(t, gatewayResources...)
}
