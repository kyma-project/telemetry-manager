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
		role:               makeMetricRole(name),
		roleBinding:        makeMetricRoleBinding(name),
	}
}

func MakeMetricGatewayRBAC(name types.NamespacedName) Rbac {
	return Rbac{
		clusterRole:        makeMetricGatewayClusterRole(name),
		clusterRoleBinding: makeClusterRoleBinding(name),
		role:               makeMetricRole(name),
		roleBinding:        makeMetricRoleBinding(name),
	}
}

func makeTraceGatewayClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	k8sAttributeRules := []rbacv1.PolicyRule{
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
	clusterRoleRules := append([]rbacv1.PolicyRule{}, k8sAttributeRules...)

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    labels.MakeDefaultLabel(name.Name),
		},
		Rules: clusterRoleRules,
	}
}

func makeMetricAgentClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	kubeletStatsRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"nodes", "nodes/stats", "nodes/proxy"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	prometheusRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"nodes", "nodes/metrics", "services", "endpoints", "pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
			Verbs:           []string{"get"},
		},
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

	clusterRoleRules := append([]rbacv1.PolicyRule{}, kubeletStatsRules...)
	clusterRoleRules = append(clusterRoleRules, prometheusRules...)
	clusterRoleRules = append(clusterRoleRules, k8sClusterRules...)

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    labels.MakeDefaultLabel(name.Name),
		},
		Rules: clusterRoleRules,
	}
}

func makeMetricGatewayClusterRole(name types.NamespacedName) *rbacv1.ClusterRole {
	k8sAttributeRules := []rbacv1.PolicyRule{
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

	kymaStatsRules := []rbacv1.PolicyRule{{
		APIGroups: []string{"operator.kyma-project.io"},
		Resources: []string{"telemetries"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"telemetry.kyma-project.io"},
		Resources: []string{"metricpipelines"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"telemetry.kyma-project.io"},
		Resources: []string{"tracepipelines"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"telemetry.kyma-project.io"},
		Resources: []string{"logpipelines"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	clusterRoleRules := append([]rbacv1.PolicyRule{}, k8sAttributeRules...)
	clusterRoleRules = append(clusterRoleRules, kymaStatsRules...)

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    labels.MakeDefaultLabel(name.Name),
		},
		Rules: clusterRoleRules,
	}
}

func makeClusterRoleBinding(name types.NamespacedName) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
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

func makeMetricRole(name types.NamespacedName) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
			Labels:    labels.MakeDefaultLabel(name.Name),
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		}}
}

func makeMetricRoleBinding(name types.NamespacedName) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
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
