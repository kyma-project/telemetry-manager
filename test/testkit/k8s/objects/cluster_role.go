package objects

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterRoleOption func(*ClusterRole)

type ClusterRole struct {
	name  string
	rules []rbacv1.PolicyRule
}

func NewClusterRole(name string, opts ...ClusterRoleOption) *ClusterRole {
	cr := &ClusterRole{
		name: name,
	}

	for _, opt := range opts {
		opt(cr)
	}

	return cr
}

func WithPolicyRule(apiGroups, resources, verbs []string) ClusterRoleOption {
	return func(cr *ClusterRole) {
		cr.rules = append(cr.rules, rbacv1.PolicyRule{
			APIGroups: apiGroups,
			Resources: resources,
			Verbs:     verbs,
		})
	}
}

func (c *ClusterRole) Name() string {
	return c.name
}

func (c *ClusterRole) K8sObject() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: c.name,
		},
		Rules: c.rules,
	}
}
