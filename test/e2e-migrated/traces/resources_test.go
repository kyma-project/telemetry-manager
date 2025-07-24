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
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestResources(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTraces)

	const (
		endpointKey   = "traces-endpoint"
		endpointValue = "http://localhost:4317"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()

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
		}
	)

	secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, endpointValue))
	pipeline := testutils.NewTracePipelineBuilder().
		WithName(pipelineName).
		WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), secret.Namespace(), endpointKey)).
		Build()

	t.Cleanup(func() {
		Expect(kitk8s.DeleteObjects(&pipeline)).To(Succeed())
	})
	Expect(kitk8s.CreateObjects(t, &pipeline, secret.K8sObject())).To(Succeed())

	assert.ResourcesExist(t, gatewayResources...)

	t.Log("When TracePipeline becomes non-reconcilable, resources should be cleaned up")
	Expect(suite.K8sClient.Delete(t.Context(), secret.K8sObject())).To(Succeed())
	assert.ResourcesNotExist(t, gatewayResources...)
}
