package otelcollector

import (
	"fmt"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sort"
	"strings"

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

		clusterRole.Rules = NormalizePolicyRules(clusterRole.Rules)

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

// WithSingletonCreatorRules returns a role option since resources needed are only namespace scoped
func WithSingletonCreatorRules() RoleOption {
	return func(r *rbacv1.Role) {
		// policy rules needed for the singletonreceivercreator component
		singletonCreatorRules := []rbacv1.PolicyRule{{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		}}
		r.Rules = append(r.Rules, singletonCreatorRules...)
	}
}

type ClusterRoleOption func(*rbacv1.ClusterRole)

func WithK8sClusterRules() ClusterRoleOption {
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

func WithKubeletStatsRules() ClusterRoleOption {
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

func WithPrometheusRules() ClusterRoleOption {
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

func WithK8sAttributeRules() ClusterRoleOption {
	// policy rules needed for the k8sattributeprocessor component
	k8sAttributeRules := []rbacv1.PolicyRule{{
		APIGroups: []string{""},
		Resources: []string{"namespaces", "pods"},
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

func WithKymaStatsRules() ClusterRoleOption {
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

type Rule struct {
	Groups        []string
	Resources     []string
	ResourceNames []string
	Verbs         []string
	URLs          []string
}

type ruleKey struct {
	Groups        string
	Resources     string
	ResourceNames string
	URLs          string
}

func (key ruleKey) String() string {
	return fmt.Sprintf("%s + %s + %s + %s", key.Groups, key.Resources, key.ResourceNames, key.URLs)
}

// ruleKeys implements sort.Interface
type ruleKeys []ruleKey

func (keys ruleKeys) Len() int           { return len(keys) }
func (keys ruleKeys) Swap(i, j int)      { keys[i], keys[j] = keys[j], keys[i] }
func (keys ruleKeys) Less(i, j int) bool { return keys[i].String() < keys[j].String() }

func (r *Rule) key() ruleKey {
	r.normalize()
	return ruleKey{
		Groups:        strings.Join(r.Groups, "&"),
		Resources:     strings.Join(r.Resources, "&"),
		ResourceNames: strings.Join(r.ResourceNames, "&"),
		URLs:          strings.Join(r.URLs, "&"),
	}
}

func (r *Rule) keyWithGroupResourceNamesURLsVerbs() string {
	key := r.key()
	verbs := strings.Join(r.Verbs, "&")
	return fmt.Sprintf("%s + %s + %s + %s", key.Groups, key.ResourceNames, key.URLs, verbs)
}

func (r *Rule) keyWithResourcesResourceNamesURLsVerbs() string {
	key := r.key()
	verbs := strings.Join(r.Verbs, "&")
	return fmt.Sprintf("%s + %s + %s + %s", key.Resources, key.ResourceNames, key.URLs, verbs)
}

func (r *Rule) keyWitGroupResourcesResourceNamesVerbs() string {
	key := r.key()
	verbs := strings.Join(r.Verbs, "&")
	return fmt.Sprintf("%s + %s + %s + %s", key.Groups, key.Resources, key.ResourceNames, verbs)
}

func (r *Rule) normalize() {
	r.Groups = removeDupAndSort(r.Groups)
	r.Resources = removeDupAndSort(r.Resources)
	r.ResourceNames = removeDupAndSort(r.ResourceNames)
	r.Verbs = removeDupAndSort(r.Verbs)
	r.URLs = removeDupAndSort(r.URLs)
}

func removeDupAndSort(strs []string) []string {
	set := make(map[string]bool)
	for _, str := range strs {
		if _, ok := set[str]; !ok {
			set[str] = true
		}
	}
	var result []string
	for str := range set {
		result = append(result, str)
	}
	sort.Strings(result)
	return result
}

func policyRuleToRule(p rbacv1.PolicyRule) *Rule {
	return &Rule{
		Groups:        p.APIGroups,
		Resources:     p.Resources,
		ResourceNames: p.ResourceNames,
		Verbs:         p.Verbs,
		URLs:          p.NonResourceURLs,
	}
}

func (r *Rule) ToPolicyRule() rbacv1.PolicyRule {
	return rbacv1.PolicyRule{
		APIGroups:       r.Groups,
		Resources:       r.Resources,
		ResourceNames:   r.ResourceNames,
		Verbs:           r.Verbs,
		NonResourceURLs: r.URLs,
	}
}

func NormalizePolicyRules(rules []rbacv1.PolicyRule) []rbacv1.PolicyRule {
	// Convert PolicyRules to internal Rules
	var internalRules []*Rule
	for _, rule := range rules {
		internalRules = append(internalRules, policyRuleToRule(rule))
	}

	ruleMap := make(map[ruleKey]*Rule)
	// First pass: merge exact matches
	for _, rule := range internalRules {
		key := rule.key()
		if _, ok := ruleMap[key]; !ok {
			ruleMap[key] = rule
			continue
		}
		existing := ruleMap[key]
		existing.Verbs = removeDupAndSort(append(existing.Verbs, rule.Verbs...))
	}

	// Deduplicate resources
	ruleMapWithoutResources := make(map[string][]*Rule)
	for _, rule := range ruleMap {
		key := rule.keyWithGroupResourceNamesURLsVerbs()
		ruleMapWithoutResources[key] = append(ruleMapWithoutResources[key], rule)
	}

	ruleMap = make(map[ruleKey]*Rule)
	for _, rules := range ruleMapWithoutResources {
		rule := rules[0]
		for _, mergeRule := range rules[1:] {
			rule.Resources = append(rule.Resources, mergeRule.Resources...)
		}
		rule.normalize()
		key := rule.key()
		ruleMap[key] = rule
	}

	// Deduplicate groups
	ruleMapWithoutGroup := make(map[string][]*Rule)
	for _, rule := range ruleMap {
		key := rule.keyWithResourcesResourceNamesURLsVerbs()
		ruleMapWithoutGroup[key] = append(ruleMapWithoutGroup[key], rule)
	}

	ruleMap = make(map[ruleKey]*Rule)
	for _, rules := range ruleMapWithoutGroup {
		rule := rules[0]
		for _, mergeRule := range rules[1:] {
			rule.Groups = append(rule.Groups, mergeRule.Groups...)
		}
		rule.normalize()
		key := rule.key()
		ruleMap[key] = rule
	}

	// Get all keys and sort them
	var keys []ruleKey
	for key := range ruleMap {
		keys = append(keys, key)
	}
	sort.Sort(ruleKeys(keys))

	// Build result in sorted order
	var result []rbacv1.PolicyRule
	for _, key := range keys {
		result = append(result, ruleMap[key].ToPolicyRule())
	}

	return result
}
