package otelcollector

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

type rbac struct {
	clusterRole        *rbacv1.ClusterRole
	clusterRoleBinding *rbacv1.ClusterRoleBinding
	role               *rbacv1.Role
	roleBinding        *rbacv1.RoleBinding
	component          string
}

type RBACOption func(*rbac, types.NamespacedName)

func newRBAC(name types.NamespacedName, componentType string, options ...RBACOption) *rbac {
	rbac := &rbac{component: componentType}

	for _, option := range options {
		option(rbac, name)
	}

	return rbac
}

func withClusterRole(options ...ClusterRoleOption) RBACOption {
	return func(r *rbac, name types.NamespacedName) {
		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    commonresources.MakeDefaultLabels(name.Name, r.component),
			},
			Rules: []rbacv1.PolicyRule{},
		}
		for _, o := range options {
			o(clusterRole)
		}

		r.clusterRole = clusterRole
	}
}

func withClusterRoleBinding() RBACOption {
	return func(r *rbac, name types.NamespacedName) {
		r.clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    commonresources.MakeDefaultLabels(name.Name, r.component),
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

func withRole(options ...RoleOption) RBACOption {
	return func(r *rbac, name types.NamespacedName) {
		role := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    commonresources.MakeDefaultLabels(name.Name, r.component),
			},
			Rules: []rbacv1.PolicyRule{},
		}

		for _, o := range options {
			o(role)
		}

		r.role = role
	}
}

func withRoleBinding() RBACOption {
	return func(r *rbac, name types.NamespacedName) {
		r.roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name.Name,
				Namespace: name.Namespace,
				Labels:    commonresources.MakeDefaultLabels(name.Name, r.component),
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

func makeTraceGatewayRBAC(namespace string) rbac {
	return *newRBAC(
		types.NamespacedName{Name: names.TraceGateway, Namespace: namespace},
		commonresources.LabelValueK8sComponentGateway,
		withClusterRole(withK8sAttributeRules()),
		withClusterRoleBinding(),
	)
}

func makeMetricAgentRBAC(namespace string) rbac {
	return *newRBAC(
		types.NamespacedName{Name: names.MetricAgent, Namespace: namespace},
		commonresources.LabelValueK8sComponentAgent,
		withClusterRole(withKubeletStatsRules(), withPrometheusRules(), withK8sClusterRules()),
		withClusterRoleBinding(),
		withRole(withLeaderElectionRules()),
		withRoleBinding(),
	)
}

func makeMetricGatewayRBAC(namespace string) rbac {
	return *newRBAC(
		types.NamespacedName{Name: names.MetricGateway, Namespace: namespace},
		commonresources.LabelValueK8sComponentGateway,
		withClusterRole(withK8sAttributeRules(), withKymaStatsRules()),
		withClusterRoleBinding(),
		withRole(withLeaderElectionRules()),
		withRoleBinding(),
	)
}

func makeLogAgentRBAC(namespace string) rbac {
	return *newRBAC(
		types.NamespacedName{Name: names.LogAgent, Namespace: namespace},
		commonresources.LabelValueK8sComponentAgent,
		withClusterRole(withK8sAttributeRules()),
		withClusterRoleBinding(),
	)
}

func makeLogGatewayRBAC(namespace string) rbac {
	return *newRBAC(
		types.NamespacedName{Name: names.LogGateway, Namespace: namespace},
		commonresources.LabelValueK8sComponentGateway,
		withClusterRole(withK8sAttributeRules()),
		withClusterRoleBinding(),
	)
}

func makeOTLPGatewayRBAC(namespace string) rbac {
	return *newRBAC(
		types.NamespacedName{Name: names.OTLPGateway, Namespace: namespace},
		commonresources.LabelValueK8sComponentGateway,
		withClusterRole(withK8sAttributeRules()),
		withClusterRoleBinding(),
	)
}

type RoleOption func(*rbacv1.Role)

// withLeaderElectionRules returns a role option since resources needed are only namespace scoped
func withLeaderElectionRules() RoleOption {
	return func(r *rbacv1.Role) {
		// policy rules needed for the leader election mechanism
		leaderElectionRules := []rbacv1.PolicyRule{{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		}}
		r.Rules = append(r.Rules, leaderElectionRules...)
	}
}

type ClusterRoleOption func(*rbacv1.ClusterRole)

func withK8sClusterRules() ClusterRoleOption {
	return func(cr *rbacv1.ClusterRole) {
		// policy rules needed for the k8sclusterreceiver component
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
		cr.Rules = append(cr.Rules, k8sClusterRules...)
	}
}

func withKubeletStatsRules() ClusterRoleOption {
	// policy rules needed for the kubeletstatsreceiver component
	kubeletStatsRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"nodes", "nodes/stats", "nodes/proxy"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, kubeletStatsRules...)
	}
}

func withPrometheusRules() ClusterRoleOption {
	// policy rules needed for the prometheusreceiver component
	prometheusRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"nodes", "nodes/metrics", "services", "endpoints", "pods"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
		Verbs:           []string{"get"},
	}}

	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, prometheusRules...)
	}
}

func withK8sAttributeRules() ClusterRoleOption {
	// policy rules needed for the k8sattributeprocessor component
	k8sAttributeRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"namespaces", "pods", "nodes"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"apps"},
		Resources: []string{"replicasets"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, k8sAttributeRules...)
	}
}

func withKymaStatsRules() ClusterRoleOption {
	// policy rules needed for the kymastatsreceiver component
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

	return func(cr *rbacv1.ClusterRole) {
		cr.Rules = append(cr.Rules, kymaStatsRules...)
	}
}
