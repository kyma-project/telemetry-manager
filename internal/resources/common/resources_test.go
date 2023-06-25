package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestMakeServiceAccount(t *testing.T) {
	name := types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "telemetry-system"}
	svcAcc := MakeServiceAccount(name)

	require.NotNil(t, svcAcc)
	require.Equal(t, svcAcc.Name, name.Name)
	require.Equal(t, svcAcc.Namespace, name.Namespace)
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
