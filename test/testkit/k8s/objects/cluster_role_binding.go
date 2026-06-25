package objects

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterRoleBindingOption func(*ClusterRoleBinding)

type ClusterRoleBinding struct {
	name     string
	subjects []rbacv1.Subject
	roleRef  rbacv1.RoleRef
}

func NewClusterRoleBinding(name string, opts ...ClusterRoleBindingOption) *ClusterRoleBinding {
	crb := &ClusterRoleBinding{
		name: name,
	}

	for _, opt := range opts {
		opt(crb)
	}

	return crb
}

func WithClusterRoleRef(roleName string) ClusterRoleBindingOption {
	return func(crb *ClusterRoleBinding) {
		crb.roleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		}
	}
}

func WithServiceAccountSubject(name, namespace string) ClusterRoleBindingOption {
	return func(crb *ClusterRoleBinding) {
		crb.subjects = append(crb.subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      name,
			Namespace: namespace,
		})
	}
}

func (c *ClusterRoleBinding) Name() string {
	return c.name
}

func (c *ClusterRoleBinding) K8sObject() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.name,
		},
		Subjects: c.subjects,
		RoleRef:  c.roleRef,
	}
}
