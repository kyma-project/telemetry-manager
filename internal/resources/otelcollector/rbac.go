package otelcollector

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type Rbac struct {
	clusterRole        *rbacv1.ClusterRole
	clusterRoleBinding *rbacv1.ClusterRoleBinding
	role               *rbacv1.Role
	roleBinding        *rbacv1.RoleBinding
}

func MakeTraceGatewayRBAC(name types.NamespacedName) Rbac {
	return Rbac{
		clusterRole:        makeTraceGatewayClusterRole(name),
		clusterRoleBinding: makeClusterRoleBinding(name),
		role:               nil,
		roleBinding:        nil,
	}
}

func MakeMetricAgentRBAC(name types.NamespacedName) Rbac {
	return Rbac{
		clusterRole:        makeMetricAgentClusterRole(name),
		clusterRoleBinding: makeClusterRoleBinding(name),
		role:               nil,
		roleBinding:        nil,
	}
}

func MakeMetricGatewayRBAC(name types.NamespacedName, kymaInputAllowed bool) Rbac {
	return Rbac{
		clusterRole:        makeMetricGatewayClusterRole(name, kymaInputAllowed),
		clusterRoleBinding: makeClusterRoleBinding(name),
		role:               makeMetricGatewayRole(name, kymaInputAllowed),
		roleBinding:        makeMetricGatewayRoleBinding(name, kymaInputAllowed),
	}
}

func makeTraceGatewayClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Rules: []rbacv1.PolicyRule{
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
		},
	}
}

func makeMetricAgentClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"nodes", "nodes/metrics", "nodes/stats", "services", "endpoints", "pods"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
				Verbs:           []string{"get"},
			},
		},
	}
}

func makeMetricGatewayClusterRole(name types.NamespacedName, kymaInputAllowed bool) *rbacv1.ClusterRole {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Rules: []rbacv1.PolicyRule{
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
		},
	}

	if kymaInputAllowed {
		clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
			APIGroups: []string{"operator.kyma-project.io"},
			Resources: []string{"telemetries"},
			Verbs:     []string{"get", "list", "watch"},
		})
	}

	k8sClusterRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"events", "namespaces", "namespaces/status", "nodes", "nodes/spec", "pods", "pods/status", "replicationcontrollers", "replicationcontrollers/status", "resourcequotas", "services"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"apps"},
		Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"extensions"},
		Resources: []string{"daemonsets", "deployments", "replicasets"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"batch"},
		Resources: []string{"jobs", "cronjobs"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"autoscaling"},
		Resources: []string{"horizontalpodautoscalers"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	clusterRole.Rules = append(clusterRole.Rules, k8sClusterRules...)

	return &clusterRole
}

func makeClusterRoleBinding(name types.NamespacedName) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Subjects: []rbacv1.Subject{{Name: name.Name, Namespace: name.Namespace, Kind: rbacv1.ServiceAccountKind}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     name.Name,
		},
	}
}

func makeMetricGatewayRole(name types.NamespacedName, kymaInputAllowed bool) *rbacv1.Role {
	if !kymaInputAllowed {
		return nil
	}

	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}
}

func makeMetricGatewayRoleBinding(name types.NamespacedName, kymaInputAllowed bool) *rbacv1.RoleBinding {
	if !kymaInputAllowed {
		return nil
	}

	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    defaultLabels(name.Name),
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
