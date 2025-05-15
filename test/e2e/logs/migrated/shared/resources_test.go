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

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
	"github.com/kyma-project/telemetry-manager/test/testkit/unique"
)

func TestResources_OTel(t *testing.T) {
	tests := []struct {
		label     string
		input     telemetryv1alpha1.LogPipelineInput
		resources []assert.Resource
	}{
		{
			label: suite.LabelLogAgent,
			input: testutils.BuildLogPipelineApplicationInput(),
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
				assert.NewResource(&corev1.ConfigMap{}, kitkyma.LogGatewayConfigMap),
				assert.NewResource(&corev1.Service{}, kitkyma.LogGatewayOTLPService),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			suite.RegisterTestCase(t, tc.label)

			const (
				endpointKey = "endpoint"
				endpoint    = "http://localhost:123"
			)

			var (
				uniquePrefix = unique.Prefix(tc.label)
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

			assert.ResourcesExist(t.Context(), suite.K8sClient, tc.resources...)
			// FIXME: This fails currently => resources are not deleted when pipeline becomes non-reconcilable
			// When pipeline becomes non-reconcilable...
			// Expect(suite.K8sClient.Delete(suite.Ctx, secret.K8sObject())).Should(Succeed())
			// assert.ResourcesNotExist(t.Context(), suite.K8sClient, tc.resources...)
		})
	}
}

func TestResources_FluentBit(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelFluentBit)

	const (
		endpointKey = "endpoint"
		endpoint    = "http://localhost:123"
	)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		secretName   = uniquePrefix()
		reources     = []assert.Resource{
			assert.NewResource(&appsv1.DaemonSet{}, kitkyma.FluentBitDaemonSetName),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.FluentBitServiceAccount),
			assert.NewResource(&rbacv1.ClusterRole{}, kitkyma.FluentBitClusterRole),
			assert.NewResource(&rbacv1.ClusterRoleBinding{}, kitkyma.FluentBitClusterRoleBinding),
			assert.NewResource(&corev1.Service{}, kitkyma.FluentBitExporterMetricsService),
			assert.NewResource(&corev1.Service{}, kitkyma.FluentBitMetricsService),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.FluentBitNetworkPolicy),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitLuaConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitParserConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitSectionsConfigMap),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.FluentBitFilesConfigMap),
		}
	)

	secret := kitk8s.NewOpaqueSecret(secretName, kitkyma.DefaultNamespaceName, kitk8s.WithStringData(endpointKey, endpoint))
	pipeline := testutils.NewLogPipelineBuilder().
		WithName(pipelineName).
		WithHTTPOutput(testutils.HTTPHostFromSecret(
			secret.Name(),
			kitkyma.DefaultNamespaceName,
			endpointKey)).
		Build()

	t.Cleanup(func() {
		require.NoError(t, kitk8s.DeleteObjects(context.Background(), suite.K8sClient, &pipeline)) //nolint:usetesting // Remove ctx from DeleteObjects
	})
	Expect(kitk8s.CreateObjects(t.Context(), suite.K8sClient, &pipeline, secret.K8sObject())).Should(Succeed())

	assert.ResourcesExist(t.Context(), suite.K8sClient, reources...)

	// When pipeline becomes non-reconcilable...
	Expect(suite.K8sClient.Delete(suite.Ctx, secret.K8sObject())).Should(Succeed())
	assert.ResourcesNotExist(t.Context(), suite.K8sClient, reources...)
}
