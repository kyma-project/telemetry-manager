package misc

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

func TestTelemetryResources(t *testing.T) {
	suite.RegisterTestCase(t, suite.LabelTelemetry)

	var (
		uniquePrefix = unique.Prefix()
		pipelineName = uniquePrefix()
		resources    = []assert.Resource{
			// Self-monitor resources
			assert.NewResource(&appsv1.Deployment{}, kitkyma.SelfMonitorName),
			assert.NewResource(&networkingv1.NetworkPolicy{}, kitkyma.SelfMonitorNetworkPolicy),
			assert.NewResource(&corev1.ServiceAccount{}, kitkyma.SelfMonitorServiceAccount),
			assert.NewResource(&rbacv1.Role{}, kitkyma.SelfMonitorRole),
			assert.NewResource(&rbacv1.RoleBinding{}, kitkyma.SelfMonitorRoleBinding),
			assert.NewResource(&corev1.ConfigMap{}, kitkyma.SelfMonitorConfigMap),
			assert.NewResource(&corev1.Service{}, kitkyma.SelfMonitorService),
		}
	)

	// Create a MetricPipeline to trigger the creation of self-monitor and its resources
	pipeline := testutils.NewMetricPipelineBuilder().
		WithName(pipelineName).
		Build()

	Expect(kitk8s.CreateObjects(t, &pipeline)).To(Succeed())

	assert.ResourcesExist(t, resources...)

	assert.ResourcesReconciled(t, resources...)
}
