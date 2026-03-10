package misc

import (
	"testing"

	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/test/testkit/assert"
	kitkyma "github.com/kyma-project/telemetry-manager/test/testkit/kyma"
	"github.com/kyma-project/telemetry-manager/test/testkit/suite"
)

func TestRBACRoles(t *testing.T) {
	suite.SetupTest(t, suite.LabelTelemetry, suite.LabelMisc)

	// Verify kyma-telemetry-view ClusterRole exists
	viewClusterRole := assert.NewResource(&rbacv1.ClusterRole{}, types.NamespacedName{Name: "kyma-telemetry-view"})
	assert.ResourcesExist(t, viewClusterRole)

	var viewRole rbacv1.ClusterRole
	err := suite.K8sClient.Get(suite.Ctx, types.NamespacedName{Name: "kyma-telemetry-view"}, &viewRole)
	Expect(err).NotTo(HaveOccurred())

	// Verify view role has correct labels for aggregation
	Expect(viewRole.Labels).To(HaveKeyWithValue("rbac.authorization.k8s.io/aggregate-to-view", "true"))

	// Verify view role has correct permissions
	Expect(viewRole.Rules).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"APIGroups": ContainElement("telemetry.kyma-project.io"),
		"Resources": And(
			ContainElement("logpipelines"),
			ContainElement("metricpipelines"),
			ContainElement("tracepipelines"),
		),
		"Verbs": And(
			ContainElement("get"),
			ContainElement("list"),
			ContainElement("watch"),
		),
	})))

	Expect(viewRole.Rules).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"APIGroups": ContainElement("operator.kyma-project.io"),
		"Resources": ContainElement("telemetries"),
		"Verbs": And(
			ContainElement("get"),
			ContainElement("list"),
			ContainElement("watch"),
		),
	})))

	// Verify kyma-telemetry-view Role exists in kyma-system namespace
	viewNamespacedRole := assert.NewResource(&rbacv1.Role{}, types.NamespacedName{Name: "kyma-telemetry-view", Namespace: kitkyma.SystemNamespaceName})
	assert.ResourcesExist(t, viewNamespacedRole)

	var nsViewRole rbacv1.Role
	err = suite.K8sClient.Get(suite.Ctx, types.NamespacedName{Name: "kyma-telemetry-view", Namespace: kitkyma.SystemNamespaceName}, &nsViewRole)
	Expect(err).NotTo(HaveOccurred())

	// Verify namespace-scoped view role has ConfigMap permissions
	Expect(nsViewRole.Rules).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"APIGroups": ContainElement(""),
		"Resources": ContainElement("configmaps"),
		"ResourceNames": And(
			ContainElement("telemetry-logpipelines"),
			ContainElement("telemetry-tracepipelines"),
			ContainElement("telemetry-metricpipelines"),
		),
		"Verbs": And(
			ContainElement("get"),
			ContainElement("list"),
			ContainElement("watch"),
		),
	})))

	// Verify kyma-telemetry-edit ClusterRole exists
	editClusterRole := assert.NewResource(&rbacv1.ClusterRole{}, types.NamespacedName{Name: "kyma-telemetry-edit"})
	assert.ResourcesExist(t, editClusterRole)

	var editRole rbacv1.ClusterRole
	err = suite.K8sClient.Get(suite.Ctx, types.NamespacedName{Name: "kyma-telemetry-edit"}, &editRole)
	Expect(err).NotTo(HaveOccurred())

	// Verify edit role has correct labels for aggregation to both edit and admin
	Expect(editRole.Labels).To(HaveKeyWithValue("rbac.authorization.k8s.io/aggregate-to-edit", "true"))
	Expect(editRole.Labels).To(HaveKeyWithValue("rbac.authorization.k8s.io/aggregate-to-admin", "true"))

	// Verify edit role has full CRUD permissions
	Expect(editRole.Rules).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"APIGroups": ContainElement("telemetry.kyma-project.io"),
		"Resources": And(
			ContainElement("logpipelines"),
			ContainElement("metricpipelines"),
			ContainElement("tracepipelines"),
		),
		"Verbs": And(
			ContainElement("create"),
			ContainElement("delete"),
			ContainElement("deletecollection"),
			ContainElement("get"),
			ContainElement("list"),
			ContainElement("patch"),
			ContainElement("update"),
			ContainElement("watch"),
		),
	})))

	Expect(editRole.Rules).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"APIGroups": ContainElement("operator.kyma-project.io"),
		"Resources": ContainElement("telemetries"),
		"Verbs": And(
			ContainElement("create"),
			ContainElement("delete"),
			ContainElement("deletecollection"),
			ContainElement("get"),
			ContainElement("list"),
			ContainElement("patch"),
			ContainElement("update"),
			ContainElement("watch"),
		),
	})))
}
