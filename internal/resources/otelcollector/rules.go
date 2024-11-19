package otelcollector

import rbacv1 "k8s.io/api/rbac/v1"

var (

	// policy rules needed for the k8sattributeprocessor component
	k8sAttributeRules = []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"namespaces", "pods"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		APIGroups: []string{"apps"},
		Resources: []string{"replicasets"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	// policy rules needed for the kubeletstatsreceiver component
	kubeletStatsRules = []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"nodes", "nodes/stats", "nodes/proxy"},
		Verbs:     []string{"get", "list", "watch"},
	}}

	// policy rules needed for the prometheusreceiver component
	prometheusRules = []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"nodes", "nodes/metrics", "services", "endpoints", "pods"},
		Verbs:     []string{"get", "list", "watch"},
	}, {
		NonResourceURLs: []string{"/metrics", "/metrics/cadvisor"},
		Verbs:           []string{"get"},
	}}

	// policy rules needed for the k8sclusterreceiver component
	k8sClusterRules = []rbacv1.PolicyRule{{
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

	// policy rules needed for the kymastatsreceiver component
	kymaStatsRules = []rbacv1.PolicyRule{{
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

	// policy rules needed for the singletonreceivercreator component
	singletonCreatorRules = []rbacv1.PolicyRule{{
		APIGroups: []string{"coordination.k8s.io"},
		Resources: []string{"leases"},
		Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
	}}
)
