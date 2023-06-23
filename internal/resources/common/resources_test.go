package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMakeServiceAccount(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	svcAcc := MakeServiceAccount(name)

	require.NotNil(t, svcAcc)
	require.Equal(t, svcAcc.Name, name.Name)
	require.Equal(t, svcAcc.Namespace, name.Namespace)
}

func TestMakeClusterRole(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	clusterRole := MakeClusterRole(name)
	expectedRules := []v1.PolicyRule{
		{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{""},
			Resources: []string{"namespaces", "pods"},
		},
		{
			Verbs:     []string{"get", "list", "watch"},
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets"},
		},
	}

	require.NotNil(t, clusterRole)
	require.Equal(t, clusterRole.Name, name.Name)
	require.Equal(t, clusterRole.Rules, expectedRules)
}

func TestMakeClusterRoleBinding(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	clusterRoleBinding := MakeClusterRoleBinding(name)
	svcAcc := MakeServiceAccount(name)

	require.NotNil(t, clusterRoleBinding)
	require.Equal(t, clusterRoleBinding.Name, name.Name)
	require.Equal(t, clusterRoleBinding.RoleRef.Name, name.Name)
	require.Equal(t, clusterRoleBinding.RoleRef.Kind, "ClusterRole")
	require.Equal(t, clusterRoleBinding.Subjects[0].Name, svcAcc.Name)
	require.Equal(t, clusterRoleBinding.Subjects[0].Kind, "ServiceAccount")

}
