package otelcollector

import (
	"testing"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMakeTraceGatewayRBAC(t *testing.T) {
	name := "test-gateway"
	namespace := "test-namespace"

	rbac := MakeTraceGatewayRBAC(types.NamespacedName{Name: name, Namespace: namespace})

	t.Run("should have a cluster role", func(t *testing.T) {
		cr := rbac.clusterRole
		expectedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}

		require.NotNil(t, cr)
		require.Equal(t, name, cr.Name)
		require.Equal(t, namespace, cr.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, cr.Labels)
		require.Equal(t, expectedRules, cr.Rules)
	})

	t.Run("should have a cluster role binding", func(t *testing.T) {
		crb := rbac.clusterRoleBinding
		checkClusterRoleBinding(t, crb, name, namespace)
	})

	t.Run("should not have a role", func(t *testing.T) {
		r := rbac.role
		require.Nil(t, r)
	})

	t.Run("should not have a role binding", func(t *testing.T) {
		rb := rbac.roleBinding
		require.Nil(t, rb)
	})
}

func TestMakeMetricAgentRBAC(t *testing.T) {
	name := "test-agent"
	namespace := "test-namespace"

	rbac := MakeMetricAgentRBAC(types.NamespacedName{Name: name, Namespace: namespace})

	t.Run("should have a cluster role", func(t *testing.T) {
		cr := rbac.clusterRole
		expectedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
				Verbs:           []string{"get"},
			},
		}

		require.NotNil(t, cr)
		require.Equal(t, cr.Name, name)
		require.Equal(t, cr.Namespace, namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, cr.Labels)
		require.Equal(t, cr.Rules, expectedRules)
	})

	t.Run("should have a cluster role binding", func(t *testing.T) {
		crb := rbac.clusterRoleBinding
		checkClusterRoleBinding(t, crb, name, namespace)
	})

	t.Run("should not have a role", func(t *testing.T) {
		r := rbac.role
		require.Nil(t, r)
	})

	t.Run("should not have a role binding", func(t *testing.T) {
		rb := rbac.roleBinding
		require.Nil(t, rb)
	})
}

func TestMakeMetricGatewayRBAC(t *testing.T) {
	name := "test-gateway"
	namespace := "test-namespace"

	rbac := MakeMetricGatewayRBAC(types.NamespacedName{Name: name, Namespace: namespace}, false)

	t.Run("should have a cluster role", func(t *testing.T) {
		cr := rbac.clusterRole
		expectedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}

		require.NotNil(t, cr)
		require.Equal(t, name, cr.Name)
		require.Equal(t, namespace, cr.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, cr.Labels)
		require.Equal(t, expectedRules, cr.Rules)
	})

	t.Run("should have a cluster role binding", func(t *testing.T) {
		crb := rbac.clusterRoleBinding
		checkClusterRoleBinding(t, crb, name, namespace)
	})

	t.Run("should not have a role", func(t *testing.T) {
		r := rbac.role
		require.Nil(t, r)
	})

	t.Run("should not have a role binding", func(t *testing.T) {
		rb := rbac.roleBinding
		require.Nil(t, rb)
	})
}

func TestMakeMetricGatewayRBACWithKymaInputAllowed(t *testing.T) {
	name := "test-gateway"
	namespace := "test-namespace"

	rbac := MakeMetricGatewayRBAC(types.NamespacedName{Name: name, Namespace: namespace}, true)

	t.Run("should have a cluster role", func(t *testing.T) {
		cr := rbac.clusterRole
		expectedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"replicasets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"operator.kyma-project.io"},
				Resources: []string{"telemetries"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}

		require.NotNil(t, cr)
		require.Equal(t, name, cr.Name)
		require.Equal(t, namespace, cr.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, cr.Labels)
		require.Equal(t, expectedRules, cr.Rules)
	})

	t.Run("should have a cluster role binding", func(t *testing.T) {
		crb := rbac.clusterRoleBinding
		checkClusterRoleBinding(t, crb, name, namespace)
	})

	t.Run("should have a role", func(t *testing.T) {
		r := rbac.role
		expectedRules := []rbacv1.PolicyRule{
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		}

		require.NotNil(t, r)
		require.Equal(t, name, r.Name)
		require.Equal(t, namespace, r.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, r.Labels)
		require.Equal(t, expectedRules, r.Rules)
	})

	t.Run("should have a role binding", func(t *testing.T) {
		rb := rbac.roleBinding
		require.NotNil(t, rb)

		require.Equal(t, name, rb.Name)
		require.Equal(t, namespace, rb.Namespace)
		require.Equal(t, map[string]string{
			"app.kubernetes.io/name": name,
		}, rb.Labels)

		subject := rb.Subjects[0]
		require.Equal(t, "ServiceAccount", subject.Kind)
		require.Equal(t, name, subject.Name)
		require.Equal(t, namespace, subject.Namespace)

		require.Equal(t, "rbac.authorization.k8s.io", rb.RoleRef.APIGroup)
		require.Equal(t, "Role", rb.RoleRef.Kind)
		require.Equal(t, name, rb.RoleRef.Name)
	})
}

func checkClusterRoleBinding(t *testing.T, crb *rbacv1.ClusterRoleBinding, name, namespace string) {
	require.NotNil(t, crb)
	require.Equal(t, name, crb.Name)
	require.Equal(t, namespace, crb.Namespace)
	require.Equal(t, map[string]string{
		"app.kubernetes.io/name": name,
	}, crb.Labels)

	subject := crb.Subjects[0]
	require.Equal(t, "ServiceAccount", subject.Kind)
	require.Equal(t, name, subject.Name)
	require.Equal(t, namespace, subject.Namespace)

	require.Equal(t, "rbac.authorization.k8s.io", crb.RoleRef.APIGroup)
	require.Equal(t, "ClusterRole", crb.RoleRef.Kind)
	require.Equal(t, name, crb.RoleRef.Name)
}
