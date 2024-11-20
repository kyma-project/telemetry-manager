package otelcollector

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/telemetry-manager/internal/labels"
)

type Rbac struct {
	clusterRole        *rbacv1.ClusterRole
	clusterRoleBinding *rbacv1.ClusterRoleBinding
	role               *rbacv1.Role
	roleBinding        *rbacv1.RoleBinding
}

type RBACOption func(*Rbac, types.NamespacedName)

func NewRBAC(name types.NamespacedName, options ...RBACOption) *Rbac {
	rbac := &Rbac{}

	for _, o := range options {
		o(rbac, name)
	}

	return rbac
}

func WithClusterRole(options ...ClusterRoleOption) RBACOption {
	return func(r *Rbac, name types.NamespacedName) {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    labels.MakeDefaultLabel(name.Name),
			},
			Rules: []rbacv1.PolicyRule{},
		}
		for _, o := range options {
			o(clusterRole)
		}

		r.clusterRole = clusterRole
	}
}

func WithClusterRoleBinding() RBACOption {
	return func(r *Rbac, name types.NamespacedName) {
		r.clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    labels.MakeDefaultLabel(name.Name),
			},
			Subjects: []rbacv1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: rbacv1.ServiceAccountKind}},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     name.Name,
			},
		}
	}
}

func WithRole(options ...RoleOption) RBACOption {
	return func(r *Rbac, name types.NamespacedName) {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    labels.MakeDefaultLabel(name.Name),
			},
			Rules: []rbacv1.PolicyRule{},
		}

		for _, o := range options {
			o(role)
		}

		r.role = role
	}
}

func WithRoleBinding() RBACOption {
	return func(r *Rbac, name types.NamespacedName) {
		r.roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    labels.MakeDefaultLabel(name.Name),
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      name.Name,
					Namespace: name.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     name.Name,
			},
		}
	}
}

func MakeTraceGatewayRBAC(name types.NamespacedName) Rbac {
	return *NewRBAC(
		name,
		WithClusterRole(WithK8sAttributeRules()),
		WithClusterRoleBinding(),
	)
}

func MakeMetricAgentRBAC(name types.NamespacedName) Rbac {
	return *NewRBAC(
		name,
		WithClusterRole(WithKubeletStatsRules(), WithPrometheusRules(), WithK8sClusterRules()),
		WithClusterRoleBinding(),
		WithRole(WithSingletonCreatorRules()),
		WithRoleBinding(),
	)
}

func MakeMetricGatewayRBAC(name types.NamespacedName) Rbac {
	return *NewRBAC(
		name,
		WithClusterRole(WithK8sAttributeRules(), WithKymaStatsRules()),
		WithClusterRoleBinding(),
		WithRole(WithSingletonCreatorRules()),
		WithRoleBinding(),
	)
}

type RoleOption func(*rbacv1.Role)

// returns a role option since resources needed are only namespace scoped
// WithSingletonCreatorRules returns a role option since resources needed are only namespace scoped
func WithSingletonCreatorRules() RoleOption {
	return func(r *rbacv1.Role) {
		r.Rules = append(r.Rules, singletonCreatorRules...)
	}
}

type ClusterRoleOption func(*rbacv1.ClusterRole)

func WithK8sClusterRules() ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, k8sClusterRules...)
	}
}

func WithKubeletStatsRules() ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, kubeletStatsRules...)
	}
}

func WithPrometheusRules() ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, prometheusRules...)
	}
}

func WithK8sAttributeRules() ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, k8sAttributeRules...)
	}
}

func WithKymaStatsRules() ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, kymaStatsRules...)
	}
}
