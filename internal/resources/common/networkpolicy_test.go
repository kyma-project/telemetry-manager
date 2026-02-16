package common

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestMakeNetworkPolicy(t *testing.T) {
	name := types.NamespacedName{Name: "test-component", Namespace: "test-namespace"}
	labels := map[string]string{"app": "test"}
	selectorLabels := map[string]string{"app.kubernetes.io/name": "test-component"}

	t.Run("creates network policy with correct metadata", func(t *testing.T) {
		np := MakeNetworkPolicy(name, labels, selectorLabels)

		require.NotNil(t, np)
		require.Equal(t, NetworkPolicyPrefix+"test-component", np.Name)
		require.Equal(t, "test-namespace", np.Namespace)
		require.Equal(t, labels, np.Labels)
		require.Equal(t, selectorLabels, np.Spec.PodSelector.MatchLabels)
	})

	t.Run("derives ingress policy type only when ingress rules present", func(t *testing.T) {
		np := MakeNetworkPolicy(name, labels, selectorLabels,
			WithIngressFromAny([]int32{8080}),
		)

		require.Len(t, np.Spec.PolicyTypes, 1)
		require.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress)
	})

	t.Run("derives egress policy type only when egress rules present", func(t *testing.T) {
		np := MakeNetworkPolicy(name, labels, selectorLabels,
			WithEgressToAny(),
		)

		require.Len(t, np.Spec.PolicyTypes, 1)
		require.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeEgress)
	})

	t.Run("derives both policy types when both rules present", func(t *testing.T) {
		np := MakeNetworkPolicy(name, labels, selectorLabels,
			WithIngressFromAny([]int32{8080}),
			WithEgressToAny(),
		)

		require.Len(t, np.Spec.PolicyTypes, 2)
		require.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeIngress)
		require.Contains(t, np.Spec.PolicyTypes, networkingv1.PolicyTypeEgress)
	})

	t.Run("has no policy types when no rules", func(t *testing.T) {
		np := MakeNetworkPolicy(name, labels, selectorLabels)

		require.Empty(t, np.Spec.PolicyTypes)
	})
}

func TestWithIngressFromAny(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("creates ingress rule allowing from any IP", func(t *testing.T) {
		np := MakeNetworkPolicy(name, nil, nil,
			WithIngressFromAny([]int32{8080, 9090}),
		)

		require.Len(t, np.Spec.Ingress, 1)
		rule := np.Spec.Ingress[0]

		require.Len(t, rule.From, 2)
		require.Equal(t, "0.0.0.0/0", rule.From[0].IPBlock.CIDR)
		require.Equal(t, "::/0", rule.From[1].IPBlock.CIDR)

		require.Len(t, rule.Ports, 2)
		require.Equal(t, int32(8080), rule.Ports[0].Port.IntVal)
		require.Equal(t, int32(9090), rule.Ports[1].Port.IntVal)
		require.Equal(t, corev1.ProtocolTCP, *rule.Ports[0].Protocol)
	})
}

func TestWithIngressFromPods(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("creates ingress rule with pod selector in same namespace", func(t *testing.T) {
		selector := map[string]string{"app": "source"}
		np := MakeNetworkPolicy(name, nil, nil,
			WithIngressFromPods(selector, []int32{8080}),
		)

		require.Len(t, np.Spec.Ingress, 1)
		rule := np.Spec.Ingress[0]

		require.Len(t, rule.From, 1)
		require.Equal(t, selector, rule.From[0].PodSelector.MatchLabels)
		require.Nil(t, rule.From[0].NamespaceSelector)

		require.Len(t, rule.Ports, 1)
		require.Equal(t, int32(8080), rule.Ports[0].Port.IntVal)
	})

	t.Run("creates ingress rule with pod selector in specific namespace", func(t *testing.T) {
		selector := map[string]string{"app": "source"}
		np := MakeNetworkPolicy(name, nil, nil,
			WithIngressFromPodsInNamespace("other-ns", selector, []int32{8080}),
		)

		require.Len(t, np.Spec.Ingress, 1)
		rule := np.Spec.Ingress[0]

		require.Len(t, rule.From, 1)
		require.Equal(t, selector, rule.From[0].PodSelector.MatchLabels)
		require.NotNil(t, rule.From[0].NamespaceSelector)
		require.Equal(t, "other-ns", rule.From[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"])
	})
}

func TestWithEgressToAny(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("creates egress rule allowing to any IP", func(t *testing.T) {
		np := MakeNetworkPolicy(name, nil, nil,
			WithEgressToAny(),
		)

		require.Len(t, np.Spec.Egress, 1)
		rule := np.Spec.Egress[0]

		require.Len(t, rule.To, 2)
		require.Equal(t, "0.0.0.0/0", rule.To[0].IPBlock.CIDR)
		require.Equal(t, "::/0", rule.To[1].IPBlock.CIDR)

		require.Empty(t, rule.Ports)
	})
}

func TestWithEgressToPods(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("creates egress rule with pod selector in same namespace", func(t *testing.T) {
		selector := map[string]string{"app": "target"}
		np := MakeNetworkPolicy(name, nil, nil,
			WithEgressToPods(selector, []int32{8080, 9090}),
		)

		require.Len(t, np.Spec.Egress, 1)
		rule := np.Spec.Egress[0]

		require.Len(t, rule.To, 1)
		require.Equal(t, selector, rule.To[0].PodSelector.MatchLabels)
		require.Nil(t, rule.To[0].NamespaceSelector)

		require.Len(t, rule.Ports, 2)
		require.Equal(t, int32(8080), rule.Ports[0].Port.IntVal)
		require.Equal(t, int32(9090), rule.Ports[1].Port.IntVal)
	})

	t.Run("creates egress rule with pod selector in specific namespace", func(t *testing.T) {
		selector := map[string]string{"app": "target"}
		np := MakeNetworkPolicy(name, nil, nil,
			WithEgressToPodsInNamespace("other-ns", selector, 8080),
		)

		require.Len(t, np.Spec.Egress, 1)
		rule := np.Spec.Egress[0]

		require.Len(t, rule.To, 1)
		require.Equal(t, selector, rule.To[0].PodSelector.MatchLabels)
		require.NotNil(t, rule.To[0].NamespaceSelector)
		require.Equal(t, "other-ns", rule.To[0].NamespaceSelector.MatchLabels["kubernetes.io/metadata.name"])
	})
}

func TestWithIngressRule(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("adds raw ingress rule", func(t *testing.T) {
		customRule := networkingv1.NetworkPolicyIngressRule{
			From: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8"}},
			},
		}

		np := MakeNetworkPolicy(name, nil, nil,
			WithIngressRule(customRule),
		)

		require.Len(t, np.Spec.Ingress, 1)
		require.Equal(t, customRule, np.Spec.Ingress[0])
	})
}

func TestWithEgressRule(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("adds raw egress rule", func(t *testing.T) {
		customRule := networkingv1.NetworkPolicyEgressRule{
			To: []networkingv1.NetworkPolicyPeer{
				{IPBlock: &networkingv1.IPBlock{CIDR: "10.0.0.0/8"}},
			},
		}

		np := MakeNetworkPolicy(name, nil, nil,
			WithEgressRule(customRule),
		)

		require.Len(t, np.Spec.Egress, 1)
		require.Equal(t, customRule, np.Spec.Egress[0])
	})
}

func TestMultipleRules(t *testing.T) {
	name := types.NamespacedName{Name: "test", Namespace: "test-ns"}

	t.Run("combines multiple ingress and egress rules", func(t *testing.T) {
		np := MakeNetworkPolicy(name, nil, nil,
			WithIngressFromAny([]int32{8080}),
			WithIngressFromPods(map[string]string{"app": "source"}, []int32{9090}),
			WithEgressToAny(),
			WithEgressToPods(map[string]string{"app": "target"}, []int32{3000}),
		)

		require.Len(t, np.Spec.Ingress, 2)
		require.Len(t, np.Spec.Egress, 2)
		require.Len(t, np.Spec.PolicyTypes, 2)
	})
}
