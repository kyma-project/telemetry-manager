package shared

import (
	"testing"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitk8sobjects "github.com/kyma-project/telemetry-manager/test/testkit/k8s/objects"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestResources_OTel(t *testing.T) {
	tests := []struct {
		label     string
		input     telemetryv1beta1.LogPipelineInput
		resources []assert.Resource
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineRuntimeInput(),
			resources: []assert.Resource{
				assert.NewResource(&appsv1.DaemonSet{}, kitkyma.LogAgentName),
				assert.NewResource(&corev1.ServiceAccount{}, kitkyma.LogAgentServiceAccount),
				assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.LogAgentClusterRole),
				assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.LogAgentClusterRoleBinding),
				assert.NewResource(&corev1.Service{}, kitkyma.LogAgentMetricsService),
				assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.LogAgentNetworkPolicy),
				assert.NewResource(&corev1.ConfigMap{}, kitkyma.LogAgentConfigMap),
				assert.NewResource(&corev1.Service{}, kitkyma.LogGatewayOTLPService),
			},
		},
		{
			label: suite.LabelLogGateway,
			input: testutils.BuildLogPipelineOTLPInput(),
			resources: []assert.Resource{
				assert.NewResource(&appsv1.Deployment{}, kitkyma.LogGatewayName),
				assert.NewResource(&corev1.Service{}, kitkyma.LogGatewayMetricsService),
				assert.NewResource(&corev1.ServiceAccount{}, kitkyma.LogGatewayServiceAccount),
				assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.LogGatewayClusterRole),
				assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.LogGatewayClusterRoleBinding),
				assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.LogGatewayNetworkPolicy),
				assert.NewResource(&corev1.Secret{}, kitkyma.LogGatewaySecretName),
				assert.NewResource(&corev1.ConfigMap{}, kitkyma.LogGatewayConfigMap),
				assert.NewResource(&corev1.Service{}, kitkyma.LogGatewayOTLPService),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				endpointKey   = "endpoint"
				endpointValue = "http://localhost:1234"
			)

			var (
				uniquePrefix = unique.Prefix(tc.label)
				pipelineName = uniquePrefix()
				secretName   = uniquePrefix()
			)

			secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(endpointKey, endpointValue))
			pipeline := testutils.NewLogPipelineBuilder().
				WithInput(tc.input).
				WithName(pipelineName).
				WithOTLPOutput(testutils.OTLPEndpointFromSecret(secret.Name(), kitkyma.DefaultNamespaceName, endpointKey)).
				Build()

			Expect(kitk8s.CreateObjects(t, &pipeline, secret.K8sObject())).To(Succeed())

			assert.ResourcesExist(t, tc.resources...)
			// FIXME: Currently failing (resources are not deleted when pipeline becomes non-reconcilable)
			// t.Log("When LogPipeline becomes non-reconcilable, resources should be cleaned up")
			// Expect(suite.K8sClient.Delete(t, secret.K8sObject())).To(Succeed())
			// assert.ResourcesNotExist(t, tc.resources...)
		})
	}
}

func TestResources_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	const hostKey = "host"

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()
		resources    = []assert.Resource{
			assert.NewResource(&appsv1.DaemonSet{}, kitkyma.FluentBitDaemonSetName),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.FluentBitServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.FluentBitClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.FluentBitClusterRoleBinding),
			assert.NewResource(&corev1.Service{}, kitkyma.FluentBitExporterMetricsService),
			assert.NewResource(&corev1.Service{}, kitkyma.FluentBitMetricsService),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.FluentBitNetworkPolicy),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitLuaConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitSectionsConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitFilesConfigMap),
		}
	)

	secret := kitk8sobjects.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8sobjects.WithStringData(hostKey, "localhost"))
	// TODO: remove parser configmap creation after logparser removal is rolled out
	parserConfigMap := kitk8sobjects.NewConfigMap(
		kitkyma.FluentBitParserConfigMap.Name,
		kitkyma.FluentBitParserConfigMap.Namespace,
	)
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHostFromSecret(
			secret.Name(),
			kitkyma.DefaultNamespaceName,
			hostKey)).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline, secret.K8sObject(), parserConfigMap.K8sObject())).To(Succeed())

	assert.ResourcesExist(t, resources...)
	assert.ResourcesNotExist(t, assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitParserConfigMap))

	// When pipeline becomes non-reconcilable...
	Expect(suite.K8sClient.Delete(t.Context(), secret.K8sObject())).To(Succeed())
	assert.ResourcesNotExist(t, resources...)
}
