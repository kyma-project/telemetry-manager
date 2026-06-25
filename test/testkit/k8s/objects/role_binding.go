package objects

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RoleBindingOption func(*RoleBinding)

type RoleBinding struct {
	name      string
	namespace string
	subjects  []rbacv1.Subject
	roleRef   rbacv1.RoleRef
}

func NewRoleBinding(name, namespace string, opts ...RoleBindingOption) *RoleBinding {
	rb := &RoleBinding{
		name:      name,
		namespace: namespace,
	}

	for _, opt := range opts {
		opt(rb)
	}

	return rb
}

func WithClusterRoleAsRoleRef(roleName string) RoleBindingOption {
	return func(rb *RoleBinding) {
		rb.roleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     roleName,
		}
	}
}

func WithServiceAccountSubjectForRole(name, namespace string) RoleBindingOption {
	return func(rb *RoleBinding) {
		rb.subjects = append(rb.subjects, rbacv1.Subject{
			Kind:      "ServiceAccount",
			Name:      name,
			Namespace: namespace,
		})
	}
}

func (r *RoleBinding) Name() string {
	return r.name
}

func (r *RoleBinding) Namespace() string {
	return r.namespace
}

func (r *RoleBinding) K8sObject() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      r.name,
			Namespace: r.namespace,
		},
		Subjects: r.subjects,
		RoleRef:  r.roleRef,
	}
}
