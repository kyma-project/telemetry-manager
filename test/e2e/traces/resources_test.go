package traces

import (
	"testing"

	. "github.com/onsi/gomega"
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

func TestResources(t *testing.T) {
	suite.SetupTestWithOptions(t, []string{suite.LabelTraces})

	const (
		endpointKey   = "traces-endpoint"
		endpointValue = "http://localhost:4317"
	)

	var (
		uniquePrefix     = unique.Prefix()
		pipelineName     = uniquePrefix()
		secretName       = uniquePrefix()
		gatewayResources = []assert.Resource{
			assert.NewResource(&appsv1.DaemonSet{}, kitkyma.OTLPGatewayName),
			assert.NewResource(&corev1.Service{}, kitkyma.TelemetryOTLPMetricsService),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.TelemetryOTLPServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.TelemetryOTLPClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.TelemetryOTLPClusterRoleBinding),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.TelemetryOTLPNetworkPolicy),
			assert.NewResource(&corev1.Secret{}, kitkyma.TelemetryOTLPSecretName),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.TelemetryOTLPConfigMap),
			assert.NewResource(&corev1.Service{}, kitkyma.TelemetryOTLPTraceService),
			// TODO(skhalash): Re-enable after fixing the istiod deployment timeout issue in the test
			// assert.NewResource(&istiosecurityclientv1.PeerAuthentication{}, kitkyma.TelemetryOTLPPeerAuthentication),
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
