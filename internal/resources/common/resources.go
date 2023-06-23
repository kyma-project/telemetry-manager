package common

import (
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func MakeServiceAccount(name types.NamespacedName) *corev1.ServiceAccount {
	serviceAccount := corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	return &serviceAccount
}

func MakeClusterRoleBinding(name types.NamespacedName) *v1.ClusterRoleBinding {
	clusterRoleBinding := v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Subjects: []v1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: "ServiceAccount"}},
		RoleRef: v1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name.Name,
		},
	}
	return &clusterRoleBinding
}

func MakeClusterRole(name types.NamespacedName) *v1.ClusterRole {
	clusterRole := v1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Rules: []v1.PolicyRule{
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
		},
	}
	return &clusterRole
}
